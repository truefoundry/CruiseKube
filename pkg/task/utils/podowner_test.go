package utils

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func ptrController(b bool) *bool { return &b }

func TestResolveRootWorkloadFromPod_DeploymentViaReplicaSet(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "web", UID: "uid-deploy"},
			Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}},
		},
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "web-abc123",
				UID:       "uid-rs",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "web",
					UID:        "uid-deploy",
					Controller: ptrController(true),
				}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "web-pod",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "web-abc123",
					UID:        "uid-rs",
					Controller: ptrController(true),
				}},
			},
		},
	)

	pod, _ := client.CoreV1().Pods("ns").Get(ctx, "web-pod", metav1.GetOptions{})
	wi, ok := ResolveRootWorkloadFromPod(ctx, client, pod)
	if !ok {
		t.Fatal("expected ok")
	}
	if wi.Kind != DeploymentKind || wi.Name != "web" {
		t.Fatalf("got %+v", wi)
	}
}

func TestResolveRootWorkloadFromPod_CronJobViaJob(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(
		&batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "tick", UID: "uid-cj"},
		},
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "tick-123",
				UID:       "uid-job",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       "tick",
					UID:        "uid-cj",
					Controller: ptrController(true),
				}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "jobpod",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "batch/v1",
					Kind:       "Job",
					Name:       "tick-123",
					UID:        "uid-job",
					Controller: ptrController(true),
				}},
			},
		},
	)

	pod, _ := client.CoreV1().Pods("ns").Get(ctx, "jobpod", metav1.GetOptions{})
	wi, ok := ResolveRootWorkloadFromPod(ctx, client, pod)
	if !ok {
		t.Fatal("expected ok")
	}
	if wi.Kind != CronJobKind || wi.Name != "tick" {
		t.Fatalf("got %+v", wi)
	}
}

func TestResolveRootWorkloadFromPod_StandaloneJob(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "once", UID: "uid-job"},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "jp",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "batch/v1",
					Kind:       "Job",
					Name:       "once",
					UID:        "uid-job",
					Controller: ptrController(true),
				}},
			},
		},
	)

	pod, _ := client.CoreV1().Pods("ns").Get(ctx, "jp", metav1.GetOptions{})
	wi, ok := ResolveRootWorkloadFromPod(ctx, client, pod)
	if !ok {
		t.Fatal("expected ok")
	}
	if wi.Kind != JobKind || wi.Name != "once" {
		t.Fatalf("got %+v", wi)
	}
}

func TestResolveRootWorkloadFromPod_StatefulSet(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "db", UID: "uid-sts"},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "db-0",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "StatefulSet",
					Name:       "db",
					UID:        "uid-sts",
					Controller: ptrController(true),
				}},
			},
		},
	)

	pod, _ := client.CoreV1().Pods("ns").Get(ctx, "db-0", metav1.GetOptions{})
	wi, ok := ResolveRootWorkloadFromPod(ctx, client, pod)
	if !ok {
		t.Fatal("expected ok")
	}
	if wi.Kind != StatefulSetKind {
		t.Fatalf("got %+v", wi)
	}
}

func TestResolveRootWorkloadFromPod_BarePod(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "solo"},
	})
	pod, _ := client.CoreV1().Pods("ns").Get(ctx, "solo", metav1.GetOptions{})
	_, ok := ResolveRootWorkloadFromPod(ctx, client, pod)
	if ok {
		t.Fatal("expected bare pod not to resolve to a workload root")
	}
}
