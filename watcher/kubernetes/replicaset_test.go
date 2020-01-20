package kuberneteswatcher_test

import (
	"context"
	kuberneteswatcher "statusbay/watcher/kubernetes"
	"testing"
	"time"

	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func createReplicasetMock(client *fake.Clientset, name string, specSelector *metav1.LabelSelector) {
	replicaset := &appsV1.ReplicaSet{
		Spec: appsV1.ReplicaSetSpec{
			Selector: specSelector,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app":               "application",
				"pod-template-hash": "pod-1",
			},
		},
	}

	client.AppsV1().ReplicaSets("pe").Create(replicaset)
}

func NewReplicasetMock(client *fake.Clientset) *kuberneteswatcher.ReplicaSetManager {

	eventManager := kuberneteswatcher.NewEventsManager(client)

	podManager := kuberneteswatcher.NewPodsManager(client, eventManager)
	replicasetManager := kuberneteswatcher.NewReplicasetManager(client, eventManager, podManager)
	podManager.Serve()
	replicasetManager.Serve()
	return replicasetManager

}

func TestReplicasetWatch(t *testing.T) {

	registry, storageMock, _ := NewRegistryMock()

	registryDeploymentData := createMockDeploymentData(registry, kuberneteswatcher.DeploymentStatusRunning)

	ctx := context.Background()

	client := fake.NewSimpleClientset()

	podManager := NewReplicasetMock(client)

	podManager.Watch <- kuberneteswatcher.WatchReplica{
		DesiredReplicas: 1,
		ListOptions:     metav1.ListOptions{},
		Registry:        registryDeploymentData,
		Namespace:       "pe",
		Ctx:             ctx,
	}
	time.Sleep(time.Second)

	specSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"pod-template-hash": "pod-1",
		},
	}
	createReplicasetMock(client, "replicaset-1", specSelector)
	createReplicasetMock(client, "replicaset-2", specSelector)
	createReplicasetMock(client, "replicaset-2", specSelector)
	time.Sleep(time.Second * 2)
	createPodMock(client, "nginx", v1.PodStatus{Phase: v1.PodFailed}, nil)
	createPodMock(client, "nginx2", v1.PodStatus{Phase: v1.PodFailed}, nil)
	time.Sleep(time.Second * 2)
	event1 := &v1.Event{Message: "message", ObjectMeta: metav1.ObjectMeta{Name: "a", CreationTimestamp: metav1.Time{Time: time.Now()}}}
	client.CoreV1().Events("pe").Create(event1)

	deployment := storageMock.MockWriteDeployment[1].Schema.Resources.Deployments["application"]

	t.Run("replicaset", func(t *testing.T) {

		if len(deployment.Replicaset) != 2 {
			t.Fatalf("unexpected replicaset watch count, got %d expected %d", len(deployment.Replicaset), 2)
		}
	})

	t.Run("pod_count", func(t *testing.T) {

		if len(deployment.Pods) != 2 {
			t.Fatalf("unexpected replicaset watch pod count, got %d expected %d", len(deployment.Replicaset), 2)
		}
	})

	t.Run("event_count", func(t *testing.T) {
		if len(*deployment.Replicaset["replicaset-1"].Events) != 1 {
			t.Fatalf("unexpected replicaset watch event count, got %d expected %d", len(*deployment.Replicaset["replicaset-1"].Events), 1)
		}
	})

}

func TestInvalidSelector(t *testing.T) {

	registry, storageMock, _ := NewRegistryMock()

	registryDeploymentData := createMockDeploymentData(registry, kuberneteswatcher.DeploymentStatusRunning)

	ctx := context.Background()

	client := fake.NewSimpleClientset()

	podManager := NewReplicasetMock(client)

	podManager.Watch <- kuberneteswatcher.WatchReplica{
		DesiredReplicas: 1,
		ListOptions:     metav1.ListOptions{},
		Registry:        registryDeploymentData,
		Namespace:       "pe",
		Ctx:             ctx,
	}
	time.Sleep(time.Second)

	specSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{},
	}
	createReplicasetMock(client, "replicaset-1", specSelector)
	time.Sleep(time.Second * 1)
	createPodMock(client, "nginx", v1.PodStatus{Phase: v1.PodFailed}, nil)
	time.Sleep(time.Second * 1)

	time.Sleep(2 * time.Second)
	deployment := storageMock.MockWriteDeployment[1].Schema.Resources.Deployments["application"]
	if len(deployment.Pods) != 0 {
		t.Fatalf("unexpected pod count watch event count, got %d expected %d", len(deployment.Pods), 0)
	}

}
