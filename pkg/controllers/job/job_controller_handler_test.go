/*
Copyright 2019 The Volcano Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package job

import (
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"testing"
	"volcano.sh/volcano/pkg/apis/helpers"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	kubeclientset "k8s.io/client-go/kubernetes"
	vkbatchv1 "volcano.sh/volcano/pkg/apis/batch/v1alpha1"
	vkv1 "volcano.sh/volcano/pkg/apis/batch/v1alpha1"
	vkbusv1 "volcano.sh/volcano/pkg/apis/bus/v1alpha1"
	kbv1 "volcano.sh/volcano/pkg/apis/scheduling/v1alpha1"
	kubebatchclient "volcano.sh/volcano/pkg/client/clientset/versioned"
	vkclientset "volcano.sh/volcano/pkg/client/clientset/versioned"
	//"volcano.sh/volcano/pkg/controllers/job"
)

func newController() *Controller {
	kubeClientSet := kubeclientset.NewForConfigOrDie(&rest.Config{
		Host: "",
		ContentConfig: rest.ContentConfig{
			GroupVersion: &v1.SchemeGroupVersion,
		},
	},
	)

	kubeBatchClientSet := kubebatchclient.NewForConfigOrDie(&rest.Config{
		Host: "",
		ContentConfig: rest.ContentConfig{
			GroupVersion: &kbv1.SchemeGroupVersion,
		},
	},
	)

	config := &rest.Config{
		Host: "",
		ContentConfig: rest.ContentConfig{
			GroupVersion: &vkv1.SchemeGroupVersion,
		},
	}

	vkclient := vkclientset.NewForConfigOrDie(config)
	controller := NewJobController(kubeClientSet, kubeBatchClientSet, vkclient, 3)

	return controller
}

func buildPod(namespace, name string, p v1.PodPhase, labels map[string]string) *v1.Pod {
	boolValue := true
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:             types.UID(fmt.Sprintf("%v-%v", namespace, name)),
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			ResourceVersion: string(uuid.NewUUID()),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: helpers.JobKind.GroupVersion().String(),
					Kind:       helpers.JobKind.Kind,
					Controller: &boolValue,
				},
			},
		},
		Status: v1.PodStatus{
			Phase: p,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}
}

func addPodAnnotation(pod *v1.Pod, annotations map[string]string) *v1.Pod {
	podWithAnnotation := pod
	for key, value := range annotations {
		if podWithAnnotation.Annotations == nil {
			podWithAnnotation.Annotations = make(map[string]string)
		}
		podWithAnnotation.Annotations[key] = value
	}
	return podWithAnnotation
}

func TestAddCommandFunc(t *testing.T) {

	namespace := "test"

	testCases := []struct {
		Name        string
		command     interface{}
		ExpectValue int
	}{
		{
			Name: "AddCommand Sucess Case",
			command: &vkbusv1.Command{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Valid Command",
					Namespace: namespace,
				},
			},
			ExpectValue: 1,
		},
		{
			Name:        "AddCommand Failure Case",
			command:     "Command",
			ExpectValue: 0,
		},
	}

	for i, testcase := range testCases {
		controller := newController()
		controller.addCommand(testcase.command)
		len := controller.commandQueue.Len()
		if testcase.ExpectValue != len {
			t.Errorf("case %d (%s): expected: %v, got %v ", i, testcase.Name, testcase.ExpectValue, len)
		}
	}
}

func TestJobAddFunc(t *testing.T) {
	namespace := "test"

	testCases := []struct {
		Name        string
		job         *vkbatchv1.Job
		ExpectValue int
	}{
		{
			Name: "AddJob Success",
			job: &vkbatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Job1",
					Namespace: namespace,
				},
			},
			ExpectValue: 1,
		},
	}
	for i, testcase := range testCases {
		controller := newController()
		controller.addJob(testcase.job)
		key := fmt.Sprintf("%s/%s", testcase.job.Namespace, testcase.job.Name)
		job, err := controller.cache.Get(key)
		if job == nil || err != nil {
			t.Errorf("Error while Adding Job in case %d with error %s", i, err)
		}
		queue := controller.getWorkerQueue(key)
		len := queue.Len()
		if testcase.ExpectValue != len {
			t.Errorf("case %d (%s): expected: %v, got %v ", i, testcase.Name, testcase.ExpectValue, len)
		}
	}
}

func TestUpdateJobFunc(t *testing.T) {
	namespace := "test"

	testcases := []struct {
		Name   string
		oldJob *vkbatchv1.Job
		newJob *vkbatchv1.Job
	}{
		{
			Name: "Job Update Success Case",
			oldJob: &vkbatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job1",
					Namespace: namespace,
				},
				Spec: vkbatchv1.JobSpec{
					SchedulerName: "volcano",
					MinAvailable:  5,
				},
				Status: vkbatchv1.JobStatus{
					State: vkbatchv1.JobState{
						Phase: vkbatchv1.Pending,
					},
				},
			},
			newJob: &vkbatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job1",
					Namespace: namespace,
				},
				Spec: vkbatchv1.JobSpec{
					SchedulerName: "volcano",
					MinAvailable:  5,
				},
				Status: vkbatchv1.JobStatus{
					State: vkbatchv1.JobState{
						Phase: vkbatchv1.Running,
					},
				},
			},
		},
		{
			Name: "Job Update Failure Case",
			oldJob: &vkbatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job1",
					Namespace: namespace,
				},
				Spec: vkbatchv1.JobSpec{
					SchedulerName: "volcano",
					MinAvailable:  5,
				},
				Status: vkbatchv1.JobStatus{
					State: vkbatchv1.JobState{
						Phase: vkbatchv1.Pending,
					},
				},
			},
			newJob: &vkbatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job1",
					Namespace: namespace,
				},
				Spec: vkbatchv1.JobSpec{
					SchedulerName: "volcano",
					MinAvailable:  5,
				},
				Status: vkbatchv1.JobStatus{
					State: vkbatchv1.JobState{
						Phase: vkbatchv1.Pending,
					},
				},
			},
		},
	}

	for i, testcase := range testcases {
		controller := newController()
		controller.addJob(testcase.oldJob)
		controller.updateJob(testcase.oldJob, testcase.newJob)
		key := fmt.Sprintf("%s/%s", testcase.newJob.Namespace, testcase.newJob.Name)
		job, err := controller.cache.Get(key)

		if job == nil || err != nil {
			t.Errorf("Error while Updating Job in case %d with error %s", i, err)
		}

		if job.Job.Status.State.Phase != testcase.newJob.Status.State.Phase {
			t.Errorf("Error while Updating Job in case %d with error %s", i, err)
		}
	}
}

func TestAddPodFunc(t *testing.T) {
	namespace := "test"

	testcases := []struct {
		Name          string
		Job           *vkbatchv1.Job
		pods          []*v1.Pod
		Annotation    map[string]string
		ExpectedValue int
	}{
		{
			Name: "AddPod Success case",
			Job: &vkbatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job1",
					Namespace: namespace,
				},
			},
			pods: []*v1.Pod{
				buildPod(namespace, "pod1", v1.PodPending, nil),
			},
			Annotation: map[string]string{
				vkbatchv1.JobNameKey:  "job1",
				vkbatchv1.JobVersion:  "0",
				vkbatchv1.TaskSpecKey: "task1",
			},
			ExpectedValue: 1,
		},
		{
			Name: "AddPod Duplicate Pod case",
			Job: &vkbatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job1",
					Namespace: namespace,
				},
			},
			pods: []*v1.Pod{
				buildPod(namespace, "pod1", v1.PodPending, nil),
				buildPod(namespace, "pod1", v1.PodPending, nil),
			},
			Annotation: map[string]string{
				vkbatchv1.JobNameKey:  "job1",
				vkbatchv1.JobVersion:  "0",
				vkbatchv1.TaskSpecKey: "task1",
			},
			ExpectedValue: 1,
		},
	}

	for i, testcase := range testcases {
		controller := newController()
		controller.addJob(testcase.Job)
		for _, pod := range testcase.pods {
			addPodAnnotation(pod, testcase.Annotation)
			controller.addPod(pod)
		}

		key := fmt.Sprintf("%s/%s", testcase.Job.Namespace, testcase.Job.Name)
		job, err := controller.cache.Get(key)

		if job == nil || err != nil {
			t.Errorf("Error while Getting Job in case %d with error %s", i, err)
		}

		var totalPods int
		for _, task := range job.Pods {
			totalPods = len(task)
		}
		if totalPods != testcase.ExpectedValue {
			t.Errorf("case %d (%s): expected: %v, got %v ", i, testcase.Name, testcase.ExpectedValue, totalPods)
		}
	}
}

func TestUpdatePodFunc(t *testing.T) {
	namespace := "test"

	testcases := []struct {
		Name          string
		Job           *vkbatchv1.Job
		oldPod        *v1.Pod
		newPod        *v1.Pod
		Annotation    map[string]string
		ExpectedValue v1.PodPhase
	}{
		{
			Name: "UpdatePod Success case",
			Job: &vkbatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job1",
					Namespace: namespace,
				},
			},
			oldPod: buildPod(namespace, "pod1", v1.PodPending, nil),
			newPod: buildPod(namespace, "pod1", v1.PodRunning, nil),
			Annotation: map[string]string{
				vkbatchv1.JobNameKey:  "job1",
				vkbatchv1.JobVersion:  "0",
				vkbatchv1.TaskSpecKey: "task1",
			},
			ExpectedValue: v1.PodRunning,
		},
		{
			Name: "UpdatePod Failed case",
			Job: &vkbatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job1",
					Namespace: namespace,
				},
			},
			oldPod: buildPod(namespace, "pod1", v1.PodPending, nil),
			newPod: buildPod(namespace, "pod1", v1.PodFailed, nil),
			Annotation: map[string]string{
				vkbatchv1.JobNameKey:  "job1",
				vkbatchv1.JobVersion:  "0",
				vkbatchv1.TaskSpecKey: "task1",
			},
			ExpectedValue: v1.PodFailed,
		},
	}

	for i, testcase := range testcases {
		controller := newController()
		controller.addJob(testcase.Job)
		addPodAnnotation(testcase.oldPod, testcase.Annotation)
		addPodAnnotation(testcase.newPod, testcase.Annotation)
		controller.addPod(testcase.oldPod)
		controller.updatePod(testcase.oldPod, testcase.newPod)

		key := fmt.Sprintf("%s/%s", testcase.Job.Namespace, testcase.Job.Name)
		job, err := controller.cache.Get(key)

		if job == nil || err != nil {
			t.Errorf("Error while Getting Job in case %d with error %s", i, err)
		}

		pod := job.Pods[testcase.Annotation[vkbatchv1.TaskSpecKey]][testcase.oldPod.Name]

		if pod.Status.Phase != testcase.ExpectedValue {
			t.Errorf("case %d (%s): expected: %v, got %v ", i, testcase.Name, testcase.ExpectedValue, pod.Status.Phase)
		}
	}
}

func TestDeletePodFunc(t *testing.T) {
	namespace := "test"

	testcases := []struct {
		Name          string
		Job           *vkbatchv1.Job
		availablePods []*v1.Pod
		deletePod     *v1.Pod
		Annotation    map[string]string
		ExpectedValue int
	}{
		{
			Name: "DeletePod success case",
			Job: &vkbatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job1",
					Namespace: namespace,
				},
			},
			availablePods: []*v1.Pod{
				buildPod(namespace, "pod1", v1.PodRunning, nil),
				buildPod(namespace, "pod2", v1.PodRunning, nil),
			},
			deletePod: buildPod(namespace, "pod2", v1.PodRunning, nil),
			Annotation: map[string]string{
				vkbatchv1.JobNameKey:  "job1",
				vkbatchv1.JobVersion:  "0",
				vkbatchv1.TaskSpecKey: "task1",
			},
			ExpectedValue: 1,
		},
		{
			Name: "DeletePod Pod NotAvailable case",
			Job: &vkbatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job1",
					Namespace: namespace,
				},
			},
			availablePods: []*v1.Pod{
				buildPod(namespace, "pod1", v1.PodRunning, nil),
				buildPod(namespace, "pod2", v1.PodRunning, nil),
			},
			deletePod: buildPod(namespace, "pod3", v1.PodRunning, nil),
			Annotation: map[string]string{
				vkbatchv1.JobNameKey:  "job1",
				vkbatchv1.JobVersion:  "0",
				vkbatchv1.TaskSpecKey: "task1",
			},
			ExpectedValue: 2,
		},
	}

	for i, testcase := range testcases {
		controller := newController()
		controller.addJob(testcase.Job)
		for _, pod := range testcase.availablePods {
			addPodAnnotation(pod, testcase.Annotation)
			controller.addPod(pod)
		}

		addPodAnnotation(testcase.deletePod, testcase.Annotation)
		controller.deletePod(testcase.deletePod)
		key := fmt.Sprintf("%s/%s", testcase.Job.Namespace, testcase.Job.Name)
		job, err := controller.cache.Get(key)

		if job == nil || err != nil {
			t.Errorf("Error while Getting Job in case %d with error %s", i, err)
		}

		var totalPods int
		for _, task := range job.Pods {
			totalPods = len(task)
		}

		if totalPods != testcase.ExpectedValue {
			t.Errorf("case %d (%s): expected: %v, got %v ", i, testcase.Name, testcase.ExpectedValue, totalPods)
		}
	}
}

func TestUpdatePodGroupFunc(t *testing.T) {

	namespace := "test"

	testCases := []struct {
		Name        string
		oldPodGroup *kbv1.PodGroup
		newPodGroup *kbv1.PodGroup
		ExpectValue int
	}{
		{
			Name: "AddCommand Sucess Case",
			oldPodGroup: &kbv1.PodGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pg1",
					Namespace: namespace,
				},
				Spec: kbv1.PodGroupSpec{
					MinMember: 3,
				},
				Status: kbv1.PodGroupStatus{
					Phase: kbv1.PodGroupPending,
				},
			},
			newPodGroup: &kbv1.PodGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pg1",
					Namespace: namespace,
				},
				Spec: kbv1.PodGroupSpec{
					MinMember: 3,
				},
				Status: kbv1.PodGroupStatus{
					Phase: kbv1.PodGroupRunning,
				},
			},
			ExpectValue: 1,
		},
	}

	for i, testcase := range testCases {
		controller := newController()
		controller.updatePodGroup(testcase.oldPodGroup, testcase.newPodGroup)
		key := fmt.Sprintf("%s/%s", testcase.oldPodGroup.Namespace, testcase.oldPodGroup.Name)
		queue := controller.getWorkerQueue(key)
		len := queue.Len()
		if testcase.ExpectValue != len {
			t.Errorf("case %d (%s): expected: %v, got %v ", i, testcase.Name, testcase.ExpectValue, len)
		}
	}
}
