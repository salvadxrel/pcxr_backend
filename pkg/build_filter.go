package pkg

import (
	"fmt"
	"pcxr/internal/app/models"
	"strings"
)

func BuildFilter(filter *models.FilterModel, startIdx int) ([]string, []any, int) {
	var wcondition []string
	var args []any
	idx := startIdx
	if len(filter.Categories) > 0 {
		placeholders := make([]string, len(filter.Categories))
		for i, cat := range filter.Categories {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, cat)
			idx++
		}
		wcondition = append(wcondition, fmt.Sprintf("category_id IN (%s)", strings.Join(placeholders, ",")))
	}
	if len(filter.Lift) > 0 {
		placeholders := make([]string, len(filter.Lift))
		for i, lift := range filter.Lift {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, lift)
			idx++
		}
		wcondition = append(wcondition, fmt.Sprintf("lifting_mechanism IN (%s)", strings.Join(placeholders, ",")))
	}
	if len(filter.Panel) > 0 {
		placeholders := make([]string, len(filter.Panel))
		for i, panel := range filter.Panel {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, panel)
			idx++
		}
		wcondition = append(wcondition, fmt.Sprintf("height_storage_console IN (%s)", strings.Join(placeholders, ",")))
	}
	if len(filter.Type_Support) > 0 {
		placeholders := make([]string, len(filter.Type_Support))
		for i, support := range filter.Type_Support {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, support)
			idx++
		}
		wcondition = append(wcondition, fmt.Sprintf("type_support IN (%s)", strings.Join(placeholders, ",")))
	}
	if filter.Price_min != nil {
		wcondition = append(wcondition, fmt.Sprintf(("price >= $%d"), idx))
		args = append(args, *filter.Price_min)
		idx++
	}
	if filter.Price_max != nil {
		wcondition = append(wcondition, fmt.Sprintf("price <=$%d", idx))
		args = append(args, *filter.Price_max)
		idx++
	}
	if filter.Frame_min != nil {
		wcondition = append(wcondition, fmt.Sprintf("min_height >= $%d", idx))
		args = append(args, *filter.Frame_min)
		idx++
	}
	if filter.Frame_max != nil {
		wcondition = append(wcondition, fmt.Sprintf("max_height <= $%d", idx))
		args = append(args, *filter.Frame_max)
		idx++
	}
	if filter.Load_capacity_min != nil {
		wcondition = append(wcondition, fmt.Sprintf("load_capacity >= $%d", idx))
		args = append(args, *filter.Load_capacity_min)
		idx++
	}
	if filter.Load_capacity_max != nil {
		wcondition = append(wcondition, fmt.Sprintf("load_capacity <= $%d", idx))
		args = append(args, *filter.Load_capacity_max)
		idx++
	}
	if filter.Frame_width_min != nil {
		wcondition = append(wcondition, fmt.Sprintf(("frame_width >= $%d"), idx))
		args = append(args, *filter.Frame_width_min)
		idx++
	}
	if filter.Frame_width_max != nil {
		wcondition = append(wcondition, fmt.Sprintf("frame_width <=$%d", idx))
		args = append(args, *filter.Frame_width_max)
		idx++
	}
	return wcondition, args, idx
}
