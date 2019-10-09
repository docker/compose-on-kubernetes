package registry

import (
	"context"
	"testing"
	"time"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	iv "github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

func generateRow(name string, services int,
	ports string, status string,
	created metav1.Time, object runtime.Object) metav1beta1.TableRow {
	return metav1beta1.TableRow{
		Object: runtime.RawExtension{Object: object},
		Cells: []interface{}{
			name, services, ports, status, created,
		},
	}
}

func TestConvertTable(t *testing.T) {
	stackWithNoSpecNoStatus := &iv.Stack{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	stackWithNoSpec := &iv.Stack{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Status: &iv.StackStatus{
			Phase:   iv.StackAvailable,
			Message: "test message",
		},
	}
	stack2Services := &iv.Stack{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test",
			CreationTimestamp: metav1.Time{Time: time.Now()},
		},
		Status: &iv.StackStatus{
			Phase:   iv.StackAvailable,
			Message: "test message",
		},
		Spec: iv.StackSpec{
			Stack: &latest.StackSpec{
				Services: []latest.ServiceConfig{
					{
						Name: "no-ports",
					},
					{
						Name: "with-ports",
						Ports: []latest.ServicePortConfig{
							{
								Target:   88,
								Protocol: "",
							},
							{
								Target:    80,
								Protocol:  "TCP",
								Published: 80,
							},
						},
					},
				},
			},
		},
	}
	cases := []struct {
		name          string
		object        runtime.Object
		expectedError string
		expectedRows  []metav1beta1.TableRow
	}{
		{
			name:          "not a stack",
			object:        &metav1beta1.Table{},
			expectedError: "unexpected object type *v1.Table",
		},
		{
			name:         "single-nospec-nostatus",
			object:       stackWithNoSpecNoStatus,
			expectedRows: []metav1beta1.TableRow{generateRow("test", 0, "", "", metav1.Time{}, stackWithNoSpecNoStatus)},
		},
		{
			name: "list-nospec-nostatus",
			object: &iv.StackList{
				Items: []iv.Stack{*stackWithNoSpecNoStatus},
			},
			expectedRows: []metav1beta1.TableRow{generateRow("test", 0, "", "", metav1.Time{}, stackWithNoSpecNoStatus)},
		},
		{
			name: "list-nospec",
			object: &iv.StackList{
				Items: []iv.Stack{*stackWithNoSpec},
			},
			expectedRows: []metav1beta1.TableRow{generateRow("test", 0, "", "Available (test message)", metav1.Time{}, stackWithNoSpec)},
		},
		{
			name:         "single",
			object:       stack2Services,
			expectedRows: []metav1beta1.TableRow{generateRow("test", 2, "with-ports: *,80", "Available (test message)", stack2Services.CreationTimestamp, stack2Services)},
		},
		{
			name: "list-mix",
			object: &iv.StackList{
				Items: []iv.Stack{*stackWithNoSpecNoStatus, *stackWithNoSpec, *stack2Services},
			},
			expectedRows: []metav1beta1.TableRow{
				generateRow("test", 0, "", "", metav1.Time{}, stackWithNoSpecNoStatus),
				generateRow("test", 0, "", "Available (test message)", metav1.Time{}, stackWithNoSpec),
				generateRow("test", 2, "with-ports: *,80", "Available (test message)", stack2Services.CreationTimestamp, stack2Services),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testee := stackTableConvertor{}
			result, err := testee.ConvertToTable(context.Background(), c.object, nil)
			if c.expectedError != "" {
				assert.EqualError(t, err, c.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if result == nil {
					return
				}
				assert.EqualValues(t, c.expectedRows, result.Rows)
			}
		})
	}
}
