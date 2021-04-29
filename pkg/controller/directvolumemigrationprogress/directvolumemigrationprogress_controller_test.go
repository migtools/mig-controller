/*
Copyright 2019 Red Hat Inc.

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

package directvolumemigrationprogress

import (
	"bytes"
	"context"
	"github.com/konveyor/mig-controller/pkg/compat"
	fakecompat "github.com/konveyor/mig-controller/pkg/compat/fake"
	"io"
	kapi "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"strings"
	"testing"
	"time"

	"github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}

//var depKey = types.NamespacedName{Name: "foo-deployment", Namespace: "default"}

const timeout = time.Second * 5

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	instance := &migapi.DirectVolumeMigrationProgress{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	// Create the DirectVolumeMigrationProgress object and expect the Reconcile
	err = c.Create(context.TODO(), instance)
	// The instance object may not be a valid object because it might be missing some required fields.
	// Please modify the instance object by adding required fields and then remove the following if statement.
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
}

func Test_parseLogs(t *testing.T) {
	type args struct {
		reader io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			args: args{reader: bytes.NewBufferString(`
          1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)2020/11/03 23:16:34 [1] <f+++++++++ file76
          1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)2020/11/03 23:16:34 [1] <f+++++++++ file76
          1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)2020/11/03 23:16:34 [1] <f+++++++++ file76
          1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)2020/11/03 23:16:34 [1] <f+++++++++ file76
          1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)2020/11/03 23:16:34 [1] <f+++++++++ file76
          1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)2020/11/03 23:16:34 [1] <f+++++++++ file76
          1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)2020/11/03 23:16:34 [1] <f+++++++++ file76
          1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)2020/11/03 23:16:34 [1] <f+++++++++ file76
          1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)2020/11/03 23:16:34 [1] <f+++++++++ file76
          1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)2020/11/03 23:16:34 [1] <f+++++++++ file76
          1.57G  89%   40.90MB/s    0:00:36 (xfr#120, to-chk=42/163)2020/11/04 01:49:41 [1] <f+++++++++ file57
          1.57G  90%   40.91MB/s    0:00:36 (xfr#121, to-chk=41/163)2020/11/04 01:49:41 [1] <f+++++++++ file58
          1.58G  90%   40.91MB/s    0:00:36 (xfr#122, to-chk=40/163)2020/11/04 01:49:41 [1] <f+++++++++ file59
          1.58G  90%   40.91MB/s    0:00:36 (xfr#123, to-chk=39/163)2020/11/04 01:49:41 [1] <f+++++++++ file60
          1.59G  90%   40.91MB/s    0:00:36 (xfr#124, to-chk=38/163)2020/11/04 01:49:42 [1] <f+++++++++ file61
          1.59G  91%   40.91MB/s    0:00:37 (xfr#125, to-chk=37/163)2020/11/04 01:49:42 [1] <f+++++++++ file62
          1.59G  91%   40.91MB/s    0:00:37 (xfr#126, to-chk=36/163)2020/11/04 01:49:42 [1] <f+++++++++ file63
          1.60G  91%   40.91MB/s    0:00:37 (xfr#127, to-chk=35/163)2020/11/04 01:49:42 [1] <f+++++++++ file64
          1.60G  91%   40.91MB/s    0:00:37 (xfr#128, to-chk=34/163)2020/11/04 01:49:42 [1] <f+++++++++ file65
          1.61G  92%   40.91MB/s    0:00:37 (xfr#129, to-chk=33/163)2020/11/04 01:49:42 [1] <f+++++++++ file66`)},
			want: strings.Join([]string{
				"1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)20",
				"1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)20",
				"1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)20",
				"1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)20",
				"1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)20",
				"1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)20",
				"1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)20",
				"1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)20",
				"1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)20",
				"1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)20",
				"1.57G  89%   40.90MB/s    0:00:36 (xfr#120, to-chk=42/163)20",
				"1.57G  90%   40.91MB/s    0:00:36 (xfr#121, to-chk=41/163)20",
				"1.58G  90%   40.91MB/s    0:00:36 (xfr#122, to-chk=40/163)20",
				"1.58G  90%   40.91MB/s    0:00:36 (xfr#123, to-chk=39/163)20",
				"1.59G  90%   40.91MB/s    0:00:36 (xfr#124, to-chk=38/163)20",
				"1.59G  91%   40.91MB/s    0:00:37 (xfr#125, to-chk=37/163)20",
				"1.59G  91%   40.91MB/s    0:00:37 (xfr#126, to-chk=36/163)20",
				"1.60G  91%   40.91MB/s    0:00:37 (xfr#127, to-chk=35/163)20",
				"1.60G  91%   40.91MB/s    0:00:37 (xfr#128, to-chk=34/163)20",
				"1.61G  92%   40.91MB/s    0:00:37 (xfr#129, to-chk=33/163)20",
			}, "\n"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLogs(tt.args.reader)
			if tt.wantErr && err == nil {
				t.Errorf("parseLogs() = want error, got a nil error")
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseLogs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReconcileDirectVolumeMigrationProgress_getCumulativeProgressPercentage(t *testing.T) {
	type fields struct {
		Owner *migapi.DirectVolumeMigrationProgress
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "given all valid distinct rsync progress percentages, should return addition of all percentages",
			fields: fields{
				Owner: &migapi.DirectVolumeMigrationProgress{
					Status: migapi.DirectVolumeMigrationProgressStatus{
						RsyncPodStatus: migapi.RsyncPodStatus{PodName: "pod-4", LastObservedProgressPercent: "10%"},
						RsyncPodStatuses: []migapi.RsyncPodStatus{
							{PodName: "pod-1", LastObservedProgressPercent: "0%"},
							{PodName: "pod-2", LastObservedProgressPercent: "10%"},
							{PodName: "pod-3", LastObservedProgressPercent: "20%"},
						},
					},
				},
			},
			want: "40%",
		},
		{
			name: "given all valid, some indistinct rsync progress percentages, should return addition of all percentages, should not count the same pod twice",
			fields: fields{
				Owner: &migapi.DirectVolumeMigrationProgress{
					Status: migapi.DirectVolumeMigrationProgressStatus{
						RsyncPodStatus: migapi.RsyncPodStatus{PodName: "pod-3", LastObservedProgressPercent: "10%"},
						RsyncPodStatuses: []migapi.RsyncPodStatus{
							{PodName: "pod-1", LastObservedProgressPercent: "0%"},
							{PodName: "pod-2", LastObservedProgressPercent: "10%"},
							{PodName: "pod-3", LastObservedProgressPercent: "10%"},
						},
					},
				},
			},
			want: "20%",
		},
		{
			name: "given some valid and some invalid, some distinct and some indistinct rsync progress percentages, should return addition of all percentages, should not count the same pod twice, should not include the invalid percentage",
			fields: fields{
				Owner: &migapi.DirectVolumeMigrationProgress{
					Status: migapi.DirectVolumeMigrationProgressStatus{
						RsyncPodStatus: migapi.RsyncPodStatus{PodName: "pod-4", LastObservedProgressPercent: "10%"},
						RsyncPodStatuses: []migapi.RsyncPodStatus{
							{PodName: "pod-1", LastObservedProgressPercent: "20%"},
							{PodName: "pod-2", LastObservedProgressPercent: "A%"},
							{PodName: "pod-3"},
							{PodName: "pod-4", LastObservedProgressPercent: "10%"},
						},
					},
				},
			},
			want: "30%",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RsyncPodProgressTask{
				Owner: tt.fields.Owner,
			}
			r.updateCumulativeProgressPercentage()
			if got := r.Owner.Status.TotalProgressPercentage; got != tt.want {
				t.Errorf("ReconcileDirectVolumeMigrationProgress.getCumulativeProgressPercentage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRsyncPodProgressTask_getCumulativeElapsedTime(t *testing.T) {
	type fields struct {
		Owner *migapi.DirectVolumeMigrationProgress
	}
	tests := []struct {
		name   string
		fields fields
		want   *metav1.Duration
	}{
		{
			name: "given empty history of Rsync pods, elapsed time should be 0",
			fields: fields{
				Owner: &migapi.DirectVolumeMigrationProgress{
					Status: migapi.DirectVolumeMigrationProgressStatus{},
				},
			},
			want: &metav1.Duration{Duration: 0},
		},
		{
			name: "given 1 pod in the history, total elapsed time should be equal to elapsed time of that pod",
			fields: fields{
				Owner: &migapi.DirectVolumeMigrationProgress{
					Status: migapi.DirectVolumeMigrationProgressStatus{
						RsyncPodStatuses: []v1alpha1.RsyncPodStatus{
							{PodName: "pod-0", ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 10}},
						},
					},
				},
			},
			want: &metav1.Duration{Duration: time.Second * 10},
		},
		{
			name: "given multiple pods in the history and one recent pod, total elapsed time should be equal to sum of elapsed times of all pods",
			fields: fields{
				Owner: &migapi.DirectVolumeMigrationProgress{
					Status: migapi.DirectVolumeMigrationProgressStatus{
						RsyncPodStatus: v1alpha1.RsyncPodStatus{
							PodName: "pod-3", ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 40},
						},
						RsyncPodStatuses: []v1alpha1.RsyncPodStatus{
							{PodName: "pod-0", ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 10}},
							{PodName: "pod-1", ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 30}},
							{PodName: "pod-2", ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 40}},
						},
					},
				},
			},
			want: &metav1.Duration{Duration: time.Second * 120},
		},
		{
			name: "given multiple pods in the history and one recent pod, one pod present in history and recent both, total elapsed time should be equal to sum of elapsed times of all pods, same pod shouldn't be counted twice",
			fields: fields{
				Owner: &migapi.DirectVolumeMigrationProgress{
					Status: migapi.DirectVolumeMigrationProgressStatus{
						RsyncPodStatus: v1alpha1.RsyncPodStatus{
							PodName: "pod-2", ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 40},
						},
						RsyncPodStatuses: []v1alpha1.RsyncPodStatus{
							{PodName: "pod-0", ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 10}},
							{PodName: "pod-1", ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 30}},
							{PodName: "pod-2", ContainerElapsedTime: &metav1.Duration{Duration: time.Second * 40}},
						},
					},
				},
			},
			want: &metav1.Duration{Duration: time.Second * 80},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RsyncPodProgressTask{
				Owner: tt.fields.Owner,
			}
			r.updateCumulativeElapsedTime()
			if got := r.Owner.Status.RsyncElapsedTime; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RsyncPodProgressTask.getCumulativeElapsedTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getLastMatch(t *testing.T) {
	matcherRegex := `\d+\%`
	type args struct {
		regex   string
		message string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "when no percentage value is present, should not return anything",
			args: args{
				regex:   matcherRegex,
				message: "",
			},
			want: "",
		},
		{
			name: "when multiple percentage values are present, should return the last match",
			args: args{
				regex:   matcherRegex,
				message: "50%\n70%\n80%\n90%",
			},
			want: "90%",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLastMatch(tt.args.regex, tt.args.message); got != tt.want {
				t.Errorf("getLastMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

type fakeGetPodLogs struct {
	podLogMessage string
	err           error
}

func (f fakeGetPodLogs) getPodLogs(pod *kapi.Pod, containerName string, tailLines *int64, previous bool) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.podLogMessage, nil
}

func TestRsyncPodProgressTask_getRsyncClientContainerStatus(t *testing.T) {
	zero := int32(0)
	one := int32(1)
	type fields struct {
		Cluster   *migapi.MigCluster
		Client    client.Client
		SrcClient compat.Client
		Owner     *migapi.DirectVolumeMigrationProgress
	}
	type args struct {
		podRef *kapi.Pod
		p      GetPodLogger
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *migapi.RsyncPodStatus
		wantErr bool
	}{
		{
			name: "Given a ready and running Pod.",
			fields: fields{
				Client:    fake.NewFakeClient(),
				SrcClient: fakecompat.NewFakeClient(),
				Cluster: &migapi.MigCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migcluster-host",
						Namespace: "openshift-migration",
					},
					Spec: migapi.MigClusterSpec{
						IsHostCluster: false,
						Insecure:      true,
					},
					Status: migapi.MigClusterStatus{
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:     "Ready",
									Status:   "True",
									Category: "Required",
								},
							},
						},
					},
				},
			},
			args: args{
				podRef: &kapi.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "dvm-rsync",
						Namespace:         "test-app-1",
						CreationTimestamp: metav1.Time{},
					},
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name: "stunnel",
							},
							{
								Name: "rsync-client",
							},
						},
					},
					Status: kapi.PodStatus{
						ContainerStatuses: []kapi.ContainerStatus{
							{
								Name: "stunnel",
								State: kapi.ContainerState{
									Running: &kapi.ContainerStateRunning{
										StartedAt: metav1.Time{},
									},
								},
							},
							{
								Name:  "rsync-client",
								Ready: true,
								State: kapi.ContainerState{
									Running: &kapi.ContainerStateRunning{
										StartedAt: metav1.Time{},
									},
								},
							},
						},
					},
				},
				p: fakeGetPodLogs{
					podLogMessage: "\"2021/04/23 16:16:14 [169] cd+++++++++ diagnostic.data/\\n427.68K   0%   11.02MB/s    0:00:00 (xfr#24, to-chk=4/31)202\\n452.92K   0%   10.80MB/s    0:00:00 (xfr#25, to-chk=3/31)202\\n2021/04/23 16:16:14 [169] cd+++++++++ journal/\\n69.69M  22%   66.13MB/s    0:00:03  \\r        105.31M  33%   \"",
					err:           nil,
				},
			},
			want: &migapi.RsyncPodStatus{
				PodName:                     "dvm-rsync",
				PodPhase:                    kapi.PodRunning,
				LogMessage:                  "\"2021/04/23 16:16:14 [169] cd+++++++++ diagnostic.data/\\n427.68K   0%   11.02MB/s    0:00:00 (xfr#24, to-chk=4/31)202\\n452.92K   0%   10.80MB/s    0:00:00 (xfr#25, to-chk=3/31)202\\n2021/04/23 16:16:14 [169] cd+++++++++ journal/\\n69.69M  22%   66.13MB/s    0:00:03  \\r        105.31M  33%   \"",
				LastObservedProgressPercent: "33%",
				LastObservedTransferRate:    "66.13MB/s",
				ExitCode:                    nil,
			},
			wantErr: false,
		},
		{
			name: "Given a non ready Pod but rsync container is terminated successfully.",
			fields: fields{
				Client:    fake.NewFakeClient(),
				SrcClient: fakecompat.NewFakeClient(),
				Cluster: &migapi.MigCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migcluster-host",
						Namespace: "openshift-migration",
					},
					Spec: migapi.MigClusterSpec{
						IsHostCluster: false,
						Insecure:      true,
					},
					Status: migapi.MigClusterStatus{
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:     "Ready",
									Status:   "True",
									Category: "Required",
								},
							},
						},
					},
				},
			},
			args: args{
				podRef: &kapi.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "dvm-rsync",
						Namespace:         "test-app-1",
						CreationTimestamp: metav1.Time{},
					},
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name: "stunnel",
							},
							{
								Name: "rsync-client",
							},
						},
					},
					Status: kapi.PodStatus{
						ContainerStatuses: []kapi.ContainerStatus{
							{
								Name: "stunnel",
								State: kapi.ContainerState{
									Running: &kapi.ContainerStateRunning{
										StartedAt: metav1.Time{},
									},
								},
							},
							{
								Name:  "rsync-client",
								Ready: false,
								LastTerminationState: kapi.ContainerState{
									Terminated: &kapi.ContainerStateTerminated{
										FinishedAt: metav1.Time{},
										ExitCode:   0,
									},
								},
							},
						},
					},
				},
				p: fakeGetPodLogs{
					podLogMessage: "",
					err:           nil,
				},
			},
			want: &migapi.RsyncPodStatus{
				PodName:                     "dvm-rsync",
				PodPhase:                    kapi.PodSucceeded,
				LogMessage:                  "",
				CreationTimestamp:           &metav1.Time{},
				LastObservedProgressPercent: "100%",
				LastObservedTransferRate:    "",
				ExitCode:                    &zero,
			},
			wantErr: false,
		},
		{
			name: "Given a succeeded Pod.",
			fields: fields{
				Client:    fake.NewFakeClient(),
				SrcClient: fakecompat.NewFakeClient(),
				Cluster: &migapi.MigCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migcluster-host",
						Namespace: "openshift-migration",
					},
					Spec: migapi.MigClusterSpec{
						IsHostCluster: false,
						Insecure:      true,
					},
					Status: migapi.MigClusterStatus{
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:     "Ready",
									Status:   "True",
									Category: "Required",
								},
							},
						},
					},
				},
			},
			args: args{
				podRef: &kapi.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "dvm-rsync",
						Namespace:         "test-app-1",
						CreationTimestamp: metav1.Time{},
					},
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name: "stunnel",
							},
							{
								Name: "rsync-client",
							},
						},
					},
					Status: kapi.PodStatus{
						Phase: kapi.PodSucceeded,
						ContainerStatuses: []kapi.ContainerStatus{
							{
								Name:  "stunnel",
								Ready: false,
								LastTerminationState: kapi.ContainerState{
									Terminated: &kapi.ContainerStateTerminated{
										FinishedAt: metav1.Time{},
										ExitCode:   0,
									},
								},
							},
							{
								Name:  "rsync-client",
								Ready: false,
								LastTerminationState: kapi.ContainerState{
									Terminated: &kapi.ContainerStateTerminated{
										FinishedAt: metav1.Time{},
										ExitCode:   0,
									},
								},
							},
						},
					},
				},
				p: fakeGetPodLogs{
					podLogMessage: "",
					err:           nil,
				},
			},
			want: &migapi.RsyncPodStatus{
				PodName:                     "dvm-rsync",
				PodPhase:                    kapi.PodSucceeded,
				LogMessage:                  "",
				CreationTimestamp:           &metav1.Time{},
				LastObservedProgressPercent: "100%",
				LastObservedTransferRate:    "",
				ExitCode:                    &zero,
			},
			wantErr: false,
		},
		{
			name: "Given a failed reference Pod.",
			fields: fields{
				Client:    fake.NewFakeClient(),
				SrcClient: fakecompat.NewFakeClient(),
				Cluster: &migapi.MigCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migcluster-host",
						Namespace: "openshift-migration",
					},
					Spec: migapi.MigClusterSpec{
						IsHostCluster: false,
						Insecure:      true,
					},
					Status: migapi.MigClusterStatus{
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:     "Ready",
									Status:   "True",
									Category: "Required",
								},
							},
						},
					},
				},
			},
			args: args{
				podRef: &kapi.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "dvm-rsync",
						Namespace:         "test-app-1",
						CreationTimestamp: metav1.Time{},
					},
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name: "stunnel",
							},
							{
								Name: "rsync-client",
							},
						},
					},
					Status: kapi.PodStatus{
						Phase: kapi.PodFailed,
						ContainerStatuses: []kapi.ContainerStatus{
							{
								Name: "stunnel",
								State: kapi.ContainerState{
									Running: &kapi.ContainerStateRunning{
										StartedAt: metav1.Time{},
									},
								},
							},
							{
								Name:  "rsync-client",
								Ready: false,
								LastTerminationState: kapi.ContainerState{
									Terminated: &kapi.ContainerStateTerminated{
										FinishedAt: metav1.Time{},
										ExitCode:   1,
									},
								},
							},
						},
					},
				},
				p: fakeGetPodLogs{
					podLogMessage: "",
					err:           nil,
				},
			},
			want: &migapi.RsyncPodStatus{
				PodName:                     "dvm-rsync",
				PodPhase:                    kapi.PodFailed,
				LogMessage:                  "",
				LastObservedProgressPercent: "",
				LastObservedTransferRate:    "",
				ExitCode:                    &one,
			},
			wantErr: false,
		},
		{
			name: "Given a non ready and failed rsync container.",
			fields: fields{
				Client:    fake.NewFakeClient(),
				SrcClient: fakecompat.NewFakeClient(),
				Cluster: &migapi.MigCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migcluster-host",
						Namespace: "openshift-migration",
					},
					Spec: migapi.MigClusterSpec{
						IsHostCluster: false,
						Insecure:      true,
					},
					Status: migapi.MigClusterStatus{
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:     "Ready",
									Status:   "True",
									Category: "Required",
								},
							},
						},
					},
				},
			},
			args: args{
				podRef: &kapi.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "dvm-rsync",
						Namespace:         "test-app-1",
						CreationTimestamp: metav1.Time{},
					},
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name: "stunnel",
							},
							{
								Name: "rsync-client",
							},
						},
					},
					Status: kapi.PodStatus{
						ContainerStatuses: []kapi.ContainerStatus{
							{
								Name:  "stunnel",
								Ready: false,
								LastTerminationState: kapi.ContainerState{
									Terminated: &kapi.ContainerStateTerminated{
										FinishedAt: metav1.Time{},
										ExitCode:   1,
									},
								},
							},
							{
								Name:  "rsync-client",
								Ready: false,
								LastTerminationState: kapi.ContainerState{
									Terminated: &kapi.ContainerStateTerminated{
										FinishedAt: metav1.Time{},
										ExitCode:   1,
									},
								},
							},
						},
					},
				},
				p: fakeGetPodLogs{
					podLogMessage: "",
					err:           nil,
				},
			},
			want: &migapi.RsyncPodStatus{
				PodName:                     "dvm-rsync",
				PodPhase:                    kapi.PodFailed,
				LogMessage:                  "",
				LastObservedProgressPercent: "",
				LastObservedTransferRate:    "",
				ExitCode:                    &one,
			},
			wantErr: false,
		},
		{
			name: "Given a non ready rsync container and refPod in pending state.",
			fields: fields{
				Client:    fake.NewFakeClient(),
				SrcClient: fakecompat.NewFakeClient(),
				Cluster: &migapi.MigCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migcluster-host",
						Namespace: "openshift-migration",
					},
					Spec: migapi.MigClusterSpec{
						IsHostCluster: false,
						Insecure:      true,
					},
					Status: migapi.MigClusterStatus{
						Conditions: migapi.Conditions{
							List: []migapi.Condition{
								{
									Type:     "Ready",
									Status:   "True",
									Category: "Required",
								},
							},
						},
					},
				},
			},
			args: args{
				podRef: &kapi.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "dvm-rsync",
						Namespace:         "test-app-1",
						CreationTimestamp: metav1.Time{},
					},
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name: "stunnel",
							},
							{
								Name: "rsync-client",
							},
						},
					},
					Status: kapi.PodStatus{
						Phase: kapi.PodPending,
						ContainerStatuses: []kapi.ContainerStatus{
							{
								Name:  "stunnel",
								Ready: false,
								State: kapi.ContainerState{
									Waiting: &kapi.ContainerStateWaiting{
										Reason: ContainerCreating,
									},
								},
							},
							{
								Name:  "rsync-client",
								Ready: false,
								State: kapi.ContainerState{
									Waiting: &kapi.ContainerStateWaiting{
										Reason: ContainerCreating,
									},
								},
							},
						},
					},
				},
				p: fakeGetPodLogs{
					podLogMessage: "",
					err:           nil,
				},
			},
			want: &migapi.RsyncPodStatus{
				PodName:                     "dvm-rsync",
				PodPhase:                    kapi.PodPending,
				LogMessage:                  "",
				LastObservedProgressPercent: "",
				LastObservedTransferRate:    "",
				ExitCode:                    nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RsyncPodProgressTask{
				Cluster:   tt.fields.Cluster,
				Client:    tt.fields.Client,
				SrcClient: tt.fields.SrcClient,
				Owner:     tt.fields.Owner,
			}
			got, err := r.getRsyncClientContainerStatus(tt.args.podRef, tt.args.p)
			if (err != nil) != tt.wantErr {
				t.Errorf("getRsyncClientContainerStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !isRsyncStatusEqual(got, tt.want) {
				t.Errorf("getRsyncClientContainerStatus() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func isRsyncStatusEqual(r1 *migapi.RsyncPodStatus, r2 *migapi.RsyncPodStatus) bool {
	if r1 == nil && r2 == nil {
		return true
	}
	if r1 == nil || r2 == nil {
		return false
	}
	if r1.PodName == r2.PodName && r1.PodPhase == r2.PodPhase && len(r1.LogMessage) == len(r2.LogMessage) && reflect.DeepEqual(r1.ExitCode, r2.ExitCode) &&
		r1.LastObservedTransferRate == r2.LastObservedTransferRate && r1.LastObservedProgressPercent == r2.LastObservedProgressPercent {
		return true
	}
	return false
}
