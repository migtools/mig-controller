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

package directpvmigrationprogress

import (
	"bytes"
	"io"
	"reflect"
	"testing"
	"time"

	migrationv1alpha1 "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
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
	instance := &migrationv1alpha1.DirectPVMigrationProgress{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}

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

	// Create the DirectPVMigrationProgress object and expect the Reconcile
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
		want    []string
		wantErr bool
	}{
		{
			args: args{reader: bytes.NewBufferString("\\r          1.63G  93%   40.95MB/s    0:00:37 (xfr#135, to-chk=27/163)2020/11/0323:16:33 [1] <f+++++++++ file72\\n\\r          1.64G  93%   40.95MB/s    0:00:38(xfr#136, to-chk=26/163)2020/11/03 23:16:33 [1] <f+++++++++ file73\\n\\r          1.64G\\ 93%   41.19MB/s    0:00:02  \\r          1.64G  93%   40.95MB/s    0:00:38 (xfr#137,to-chk=25/163)2020/11/03 23:16:34 [1] <f+++++++++ file74\\n\\r          1.64G  94%\\  40.96MB/s    0:00:38 (xfr#138, to-chk=24/163)2020/11/03 23:16:34 [1] <f+++++++++file75\\n\\r          1.65G  94%   40.95MB/s    0:00:38 (xfr#139, to-chk=23/163)2020/11/0323:16:34 [1] <f+++++++++ file76\\n\\r          1.65G  94%   40.95MB/s    0:00:38(xfr#140, to-chk=22/163)2020/11/03 23:16:34 [1] <f+++++++++ file77\\n\\r          1.66G\\ 94%   40.95MB/s    0:00:38 (xfr#141, to-chk=21/163)2020/11/03 23:16:34 [1] <f+++++++++file78\\n\\r          1.66G  95%   40.95MB/s    0:00:38 (xfr#142, to-chk=20/163)2020/11/0323:16:34 [1] <f+++++++++ file79\\n\\r          1.67G  95%   40.95MB/s    0:00:38(xfr#143, to-chk=19/163)2020/11/03 23:16:34 [1] <f+++++++++ file80\\n\\r          1.67G\\ 95%   40.96MB/s    0:00:38 (xfr#144, to-chk=18/163)2020/11/03 23:16:34 [1] <f+++++++++file81\\n")},
			want: []string{
				"          1.71G  97%   41.06MB/s    0:00:39 (xfr#153, to-chk=9/163)",
				"    2020/11/03 22:09:03 [1] <f+++++++++ file90",
				"    \\   0:00:39 (xfr#154, to-chk=8/163)2020/11/03 22:09:03 [1] <f+++++++++ file91",
			},
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
