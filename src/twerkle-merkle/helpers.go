package twerkle_merkle

import (
	"fmt"
	"reflect"
	"strings"
)

// PrintStructTable takes any data and returns a markdown table of its fields with adjusted column widths
func PrintStructTable(data any) string {
	var builder strings.Builder

	// Get the reflection value and type of the input
	val := reflect.ValueOf(data)
	typ := reflect.TypeOf(data)

	// Ensure the input is a struct
	if val.Kind() != reflect.Struct {
		return "Error: Input must be a struct"
	}

	// Initialize max widths for columns
	maxNameTypeWidth := len("Variable Name (Type)")
	maxValueWidth := len("Value")

	// Collect field data and calculate max widths
	type fieldInfo struct {
		name, typ, value string
	}
	var fields []fieldInfo

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		// Get field name, type, and value
		name := field.Name
		typStr := field.Type.String()
		value := fmt.Sprintf("%v", fieldValue.Interface())

		// Format name and type together
		nameType := fmt.Sprintf("%s (%s)", name, typStr)

		// Update max widths
		if len(nameType) > maxNameTypeWidth {
			maxNameTypeWidth = len(nameType)
		}
		if len(value) > maxValueWidth {
			maxValueWidth = len(value)
		}

		fields = append(fields, fieldInfo{nameType, typStr, value})
	}

	// Write table header
	header := fmt.Sprintf("| %-*s | %-*s |\n", maxNameTypeWidth, "Variable Name (Type)", maxValueWidth, "Value")
	builder.WriteString(header)

	// Write separator
	separator := fmt.Sprintf("| %s | %s |\n", strings.Repeat("-", maxNameTypeWidth), strings.Repeat("-", maxValueWidth))
	builder.WriteString(separator)

	// Write table rows
	for _, f := range fields {
		builder.WriteString(fmt.Sprintf("| %-*s | %-*s |\n", maxNameTypeWidth, f.name, maxValueWidth, f.value))
	}

	return builder.String()
}
