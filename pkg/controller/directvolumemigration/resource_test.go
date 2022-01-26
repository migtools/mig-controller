package directvolumemigration

import (
	"reflect"
	"testing"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	fakecompat "github.com/konveyor/mig-controller/pkg/compat/fake"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestTask_getLimitRangeForNamespace(t *testing.T) {
	type args struct {
		ns     string
		client k8sclient.Client
	}
	tests := []struct {
		name    string
		args    args
		want    *corev1.LimitRange
		wantErr bool
	}{
		{
			name: "when no limit ranges present for a namespace, must return nil",
			args: args{
				ns:     "test",
				client: getFakeCompatClient(),
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "when 1 limit range is present for a namespace, must return that range",
			args: args{
				ns: "test",
				client: getFakeCompatClient(
					&corev1.LimitRange{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "range-1"},
						Spec: corev1.LimitRangeSpec{
							Limits: []corev1.LimitRangeItem{
								{
									Max: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
									Type: corev1.LimitTypeContainer,
								},
								{
									Max: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("200m"),
										corev1.ResourceMemory: resource.MustParse("2Gi"),
									},
									Type: corev1.LimitTypePod,
								},
							},
						},
					},
				),
			},
			want: &corev1.LimitRange{
				Spec: corev1.LimitRangeSpec{
					Limits: []corev1.LimitRangeItem{
						{
							Max: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
							Type: corev1.LimitTypePod,
						},
						{
							Max: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Type: corev1.LimitTypeContainer,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "when multiple limit ranges are present for a namespace, must return the most restrictive range",
			args: args{
				ns: "test",
				client: getFakeCompatClient(
					&corev1.LimitRange{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "range-1"},
						Spec: corev1.LimitRangeSpec{
							Limits: []corev1.LimitRangeItem{
								{
									Max: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("200m"),
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
									Min: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Type: corev1.LimitTypeContainer,
								},
								{
									Max: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("400m"),
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
									Min: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("200m"),
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
									Type: corev1.LimitTypePod,
								},
							},
						},
					},
					&corev1.LimitRange{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "range-2"},
						Spec: corev1.LimitRangeSpec{
							Limits: []corev1.LimitRangeItem{
								{
									Max: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("800m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Min: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("200m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Type: corev1.LimitTypeContainer,
								},
								{
									Max: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("500m"),
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
									Min: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("300m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Type: corev1.LimitTypePod,
								},
							},
						},
					},
				),
			},
			want: &corev1.LimitRange{
				Spec: corev1.LimitRangeSpec{
					Limits: []corev1.LimitRangeItem{
						{
							Max: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("400m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
							Min: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("300m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
							Type: corev1.LimitTypePod,
						},
						{
							Max: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Min: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Type: corev1.LimitTypeContainer,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "when multiple limit ranges are present for a namespace with some of the limit values as nil, must return the most restrictive range, should not consider nil values as zero",
			args: args{
				ns: "test",
				client: getFakeCompatClient(
					&corev1.LimitRange{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "range-1"},
						Spec: corev1.LimitRangeSpec{
							Limits: []corev1.LimitRangeItem{
								{
									Max: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
									Min: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Type: corev1.LimitTypeContainer,
								},
								{
									Max: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
									Min: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
									Type: corev1.LimitTypePod,
								},
							},
						},
					},
					&corev1.LimitRange{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "range-2"},
						Spec: corev1.LimitRangeSpec{
							Limits: []corev1.LimitRangeItem{
								{
									Max: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Min: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Type: corev1.LimitTypeContainer,
								},
								{
									Max: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
									Min: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Type: corev1.LimitTypePod,
								},
							},
						},
					},
				),
			},
			want: &corev1.LimitRange{
				Spec: corev1.LimitRangeSpec{
					Limits: []corev1.LimitRangeItem{
						{
							Max: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
							Min: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
							Type: corev1.LimitTypePod,
						},
						{
							Max: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Min: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Type: corev1.LimitTypeContainer,
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Task{}
			got, err := tr.getLimitRangeForNamespace(tt.args.ns, tt.args.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("Task.getLimitRangeForNamespace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Task.getLimitRangeForNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTask_getUserConfiguredResourceRequirements(t *testing.T) {
	getClient := func(obj ...k8sclient.Object) k8sclient.Client {
		client, _ := fakecompat.NewFakeClient(obj...)
		return client
	}
	type args struct {
		cpuLimit    string
		memoryLimit string
		cpuRequests string
		memRequests string
	}
	tests := []struct {
		name    string
		client  k8sclient.Client
		args    args
		want    corev1.ResourceRequirements
		wantErr bool
	}{
		{
			name: "when no resource requirements are configured, should return default values for all resources",
			client: getClient(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migration-controller",
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Data: map[string]string{},
				},
			),
			args: args{
				cpuLimit:    TRANSFER_POD_CPU_LIMIT,
				memoryLimit: TRANSFER_POD_MEMORY_LIMIT,
				cpuRequests: TRANSFER_POD_CPU_REQUESTS,
				memRequests: TRANSFER_POD_MEMORY_REQUESTS,
			},
			want: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
					corev1.ResourceCPU:    resource.MustParse("400m"),
				},
				Limits: corev1.ResourceList{},
			},
			wantErr: false,
		},
		{
			name: "when some resource requirements are configured, should return default values for all resources that are not configured by user",
			client: getClient(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migration-controller",
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Data: map[string]string{
						TRANSFER_POD_CPU_LIMIT:       "1",
						TRANSFER_POD_MEMORY_REQUESTS: "256Mi",
					},
				},
			),
			args: args{
				cpuLimit:    TRANSFER_POD_CPU_LIMIT,
				memoryLimit: TRANSFER_POD_MEMORY_LIMIT,
				cpuRequests: TRANSFER_POD_CPU_REQUESTS,
				memRequests: TRANSFER_POD_MEMORY_REQUESTS,
			},
			want: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("256Mi"),
					corev1.ResourceCPU:    resource.MustParse("400m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
				},
			},
			wantErr: false,
		},
		{
			name: "when all resource requirements are configured, should return all values configured by user",
			client: getClient(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "migration-controller",
						Namespace: migapi.OpenshiftMigrationNamespace,
					},
					Data: map[string]string{
						TRANSFER_POD_CPU_LIMIT:       "1",
						TRANSFER_POD_MEMORY_REQUESTS: "128Mi",
						TRANSFER_POD_CPU_REQUESTS:    "100m",
						TRANSFER_POD_MEMORY_LIMIT:    "512Mi",
					},
				},
			),
			args: args{
				cpuLimit:    TRANSFER_POD_CPU_LIMIT,
				memoryLimit: TRANSFER_POD_MEMORY_LIMIT,
				cpuRequests: TRANSFER_POD_CPU_REQUESTS,
				memRequests: TRANSFER_POD_MEMORY_REQUESTS,
			},
			want: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("128Mi"),
					corev1.ResourceCPU:    resource.MustParse("100m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Task{
				Client: tt.client,
			}
			got, err := tr.getUserConfiguredResourceRequirements(tt.args.cpuLimit, tt.args.memoryLimit, tt.args.cpuRequests, tt.args.memRequests)
			if (err != nil) != tt.wantErr {
				t.Errorf("Task.getUserConfiguredResourceRequirements() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Task.getUserConfiguredResourceRequirements() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_applyLimitRangeOnRequirements(t *testing.T) {
	type args struct {
		requirements *corev1.ResourceRequirements
		limitRange   corev1.LimitRange
	}
	tests := []struct {
		name string
		args args
		want *corev1.ResourceRequirements
	}{
		{
			name: "when no limit range is present, must return the original requirements",
			args: args{
				requirements: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("400m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("800m"),
						corev1.ResourceMemory: resource.MustParse("1024Mi"),
					},
				},
				limitRange: corev1.LimitRange{},
			},
			want: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("400m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("800m"),
					corev1.ResourceMemory: resource.MustParse("1024Mi"),
				},
			},
		},
		{
			name: "when Max container limit range is present, the final requirement must not exceed the max limit",
			args: args{
				requirements: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("400m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
				limitRange: corev1.LimitRange{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: corev1.LimitTypeContainer,
								Max: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
			want: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		},
		{
			name: "when Max container limit range is present, the final requirement must not exceed the max limit",
			args: args{
				requirements: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("400m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
				limitRange: corev1.LimitRange{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: corev1.LimitTypeContainer,
								Max: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
			want: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		},
		{
			name: "when Min container limit range is present, the final requirement must be at least the allowed Min",
			args: args{
				requirements: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("400m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("400m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
				limitRange: corev1.LimitRange{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: corev1.LimitTypeContainer,
								Min: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
			want: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("400m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("400m"),
				},
			},
		},
		{
			name: "when Min container limit range is present, the final requirement must be at least the allowed Min",
			args: args{
				requirements: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
				limitRange: corev1.LimitRange{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: corev1.LimitTypeContainer,
								Min: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
			want: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		},
		{
			name: "when Max pod limit range is present, the final requirement must not exceed the scaled Max",
			args: args{
				requirements: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("300m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
				limitRange: corev1.LimitRange{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: corev1.LimitTypePod,
								Max: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("400m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
				},
			},
			want: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		},
		{
			name: "when Max pod limit range is present with some nil values, the final requirement must not exceed the scaled Max, the nil value must not be considered zero",
			args: args{
				requirements: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("300m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
				limitRange: corev1.LimitRange{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: corev1.LimitTypePod,
								Max: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
				},
			},
			want: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("300m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyLimitRangeOnRequirements(tt.args.requirements, tt.args.limitRange)
		})
		isEqual := func(a, b *corev1.ResourceRequirements) bool {
			if a == nil || b == nil {
				return a == b
			} else {
				if a.Requests == nil || b.Requests == nil {
					return (a.Requests == nil) && (b.Requests == nil)
				}
				v1, v2 := a.Requests[corev1.ResourceCPU], b.Requests[corev1.ResourceCPU]
				if v1.Cmp(v2) != 0 {
					return false
				}
				v1, v2 = a.Requests[corev1.ResourceMemory], b.Requests[corev1.ResourceMemory]
				if v1.Cmp(v2) != 0 {
					return false
				}
				if a.Limits == nil || b.Limits == nil {
					return (a.Limits == nil) && (b.Limits == nil)
				}
				v1, v2 = a.Limits[corev1.ResourceCPU], b.Limits[corev1.ResourceCPU]
				if v1.Cmp(v2) != 0 {
					return false
				}
				v1, v2 = a.Limits[corev1.ResourceMemory], b.Limits[corev1.ResourceMemory]
				if v1.Cmp(v2) != 0 {
					return false
				}
			}
			return true
		}
		if !isEqual(tt.args.requirements, tt.want) {
			t.Errorf("Task.applyLimitRangeOnRequirements() = %v, want %v", tt.args.requirements, tt.want)
		}
	}
}

func Test_getScaledQuantity(t *testing.T) {
	// memory quantities
	oneGi := resource.MustParse("1Gi")
	twoGi := resource.MustParse("2Gi")
	oneHundredMi := resource.MustParse("100Mi")
	twoHundredMi := resource.MustParse("200Mi")
	fiveOneTwoMi := resource.MustParse("512Mi")
	// cpu quantities
	twoHundredMilli := resource.MustParse("200m")
	fourHundredMilli := resource.MustParse("400m")
	fiveHundredMilli := resource.MustParse("500m")
	oneThousandMilli := resource.MustParse("1")
	sixHundredMilli := resource.MustParse("600m")
	twelveHundredMilli := resource.MustParse("1200m")
	type args struct {
		q     *resource.Quantity
		scale int64
	}
	tests := []struct {
		name string
		args args
		want *resource.Quantity
	}{
		{
			name: "given a nil quantity, should return nil quantity",
			args: args{
				q:     nil,
				scale: 2,
			},
			want: nil,
		},
		{
			name: "given a scale less than 1, should return nil quantity",
			args: args{
				q:     &twoHundredMi,
				scale: 0,
			},
			want: nil,
		},
		{
			name: "given a memory quantity, should return scaled down quantity",
			args: args{
				q:     &twoHundredMi,
				scale: 2,
			},
			want: &oneHundredMi,
		},
		{
			name: "given a memory quantity, should return scaled down quantity",
			args: args{
				q:     &twoGi,
				scale: 2,
			},
			want: &oneGi,
		},
		{
			name: "given a memory quantity, should return scaled down quantity",
			args: args{
				q:     &oneGi,
				scale: 2,
			},
			want: &fiveOneTwoMi,
		},
		{
			name: "given a CPU whole quantity, should return scaled down milli quantity",
			args: args{
				q:     &oneThousandMilli,
				scale: 2,
			},
			want: &fiveHundredMilli,
		},
		{
			name: "given a CPU milli quantity, should return scaled down quantity",
			args: args{
				q:     &fourHundredMilli,
				scale: 2,
			},
			want: &twoHundredMilli,
		},
		{
			name: "given a CPU milli quantity, should return scaled down quantity",
			args: args{
				q:     &twelveHundredMilli,
				scale: 2,
			},
			want: &sixHundredMilli,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getScaledDownQuantity(tt.args.q, tt.args.scale)
			if (got != nil && tt.want != nil && got.Cmp(*tt.want) != 0) || (tt.want == nil && got != nil) {
				t.Errorf("getScaledQuantity() = %v, want %v", got, tt.want)
			}
		})
	}
}
