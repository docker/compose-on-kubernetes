package registry

import (
	"context"
	"fmt"
	"strconv"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	iv "github.com/docker/compose-on-kubernetes/internal/internalversion"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

type stackTableConvertor struct {
}

func (c stackTableConvertor) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1beta1.Table, error) {
	var (
		table metav1beta1.Table
		items []iv.Stack
	)
	if lst, ok := object.(*iv.StackList); ok {
		table.ResourceVersion = lst.ResourceVersion
		table.SelfLink = lst.SelfLink
		table.Continue = lst.Continue
		items = lst.Items
	} else if item, ok := object.(*iv.Stack); ok {
		table.ResourceVersion = item.ResourceVersion
		table.SelfLink = item.SelfLink
		items = []iv.Stack{*item}
	} else {
		return nil, fmt.Errorf("unexpected object type %T", object)
	}
	table.ColumnDefinitions = []metav1beta1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "Stack name"},
		{Name: "Services", Type: "int", Description: "Number of services"},
		{Name: "Ports", Type: "string", Description: "Exposed ports"},
		{Name: "Status", Type: "string", Description: "Current stack status"},
		{Name: "Created At", Type: "date", Description: "Creation date"},
	}
	for _, item := range items {
		local := item
		serviceCount := 0
		ports := ""
		if item.Spec.Stack != nil {
			serviceCount = len(item.Spec.Stack.Services)
			ports = extractPortsSummary(item.Spec.Stack)
		}
		status := ""
		if item.Status != nil {
			status = fmt.Sprintf("%s (%s)", item.Status.Phase, item.Status.Message)
		}
		table.Rows = append(table.Rows, metav1beta1.TableRow{
			Object: runtime.RawExtension{Object: &local},
			Cells: []interface{}{
				item.Name,
				serviceCount,
				ports,
				status,
				item.CreationTimestamp,
			},
		})
	}
	return &table, nil
}

func extractPortsSummary(s *latest.StackSpec) string {
	ports := ""
	for _, svc := range s.Services {
		if len(svc.Ports) > 0 {
			if ports != "" {
				ports += ", "
			}
			ports += fmt.Sprintf("%s: ", svc.Name)
			for portix, port := range svc.Ports {
				if portix != 0 {
					ports += ","
				}
				if port.Published == 0 {
					ports += "*"
				} else {
					ports += strconv.FormatInt(int64(port.Published), 10)
				}
			}
		}
	}
	return ports
}
