package witgo

import (
	"encoding/json"
	"reflect"
)

type ValueLimits struct {
	MaxArgumentBytes uint64
	MaxResultBytes   uint64
	MaxCollectionLen uint64
	MaxValueDepth    uint32
	MaxStringBytes   uint64
}

func validateArguments(values []any, limits ValueLimits) error {
	visited := make(map[valueVisit]bool)
	for _, value := range values {
		if err := validateRuntimeValue(reflect.ValueOf(value), limits, 1, visited); err != nil {
			return err
		}
	}
	if limits.MaxArgumentBytes > 0 {
		data, err := json.Marshal(values)
		if err != nil { return err }
		if uint64(len(data)) > limits.MaxArgumentBytes {
			return &RuntimeLimitError{Limit: "MaxArgumentBytes", Maximum: limits.MaxArgumentBytes, Actual: uint64(len(data)), Cause: ErrArgumentTooLarge}
		}
	}
	return nil
}

type valueVisit struct { typ reflect.Type; pointer uintptr }

func validateRuntimeValue(value reflect.Value, limits ValueLimits, depth uint32, visited map[valueVisit]bool) error {
	if !value.IsValid() { return nil }
	if limits.MaxValueDepth > 0 && depth > limits.MaxValueDepth {
		return &RuntimeLimitError{Limit: "MaxValueDepth", Maximum: uint64(limits.MaxValueDepth), Actual: uint64(depth), Cause: ErrValueDepthExceeded}
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() { return nil }
		if value.Kind() == reflect.Pointer {
			visit := valueVisit{typ: value.Type(), pointer: value.Pointer()}
			if visited[visit] { return &RuntimeLimitError{Limit: "MaxValueDepth", Maximum: uint64(limits.MaxValueDepth), Actual: uint64(depth + 1), Cause: ErrValueDepthExceeded} }
			visited[visit] = true
			defer delete(visited, visit)
		}
		value = value.Elem()
		depth++
		if limits.MaxValueDepth > 0 && depth > limits.MaxValueDepth {
			return &RuntimeLimitError{Limit: "MaxValueDepth", Maximum: uint64(limits.MaxValueDepth), Actual: uint64(depth), Cause: ErrValueDepthExceeded}
		}
	}
	switch value.Kind() {
	case reflect.String:
		if limits.MaxStringBytes > 0 && uint64(value.Len()) > limits.MaxStringBytes {
			return &RuntimeLimitError{Limit: "MaxStringBytes", Maximum: limits.MaxStringBytes, Actual: uint64(value.Len()), Cause: ErrArgumentTooLarge}
		}
	case reflect.Slice, reflect.Array:
		if limits.MaxCollectionLen > 0 && uint64(value.Len()) > limits.MaxCollectionLen {
			return &RuntimeLimitError{Limit: "MaxCollectionLen", Maximum: limits.MaxCollectionLen, Actual: uint64(value.Len()), Cause: ErrArgumentTooLarge}
		}
		if value.Kind() == reflect.Slice && !value.IsNil() {
			visit := valueVisit{typ: value.Type(), pointer: value.Pointer()}
			if visited[visit] { return &RuntimeLimitError{Limit: "MaxValueDepth", Maximum: uint64(limits.MaxValueDepth), Actual: uint64(depth + 1), Cause: ErrValueDepthExceeded} }
			visited[visit] = true
			defer delete(visited, visit)
		}
		for index := 0; index < value.Len(); index++ {
			if err := validateRuntimeValue(value.Index(index), limits, depth+1, visited); err != nil { return err }
		}
	case reflect.Map:
		if value.IsNil() { return nil }
		if limits.MaxCollectionLen > 0 && uint64(value.Len()) > limits.MaxCollectionLen {
			return &RuntimeLimitError{Limit: "MaxCollectionLen", Maximum: limits.MaxCollectionLen, Actual: uint64(value.Len()), Cause: ErrArgumentTooLarge}
		}
		visit := valueVisit{typ: value.Type(), pointer: uintptr(value.UnsafePointer())}
		if visited[visit] { return &RuntimeLimitError{Limit: "MaxValueDepth", Maximum: uint64(limits.MaxValueDepth), Actual: uint64(depth + 1), Cause: ErrValueDepthExceeded} }
		visited[visit] = true
		defer delete(visited, visit)
		iterator := value.MapRange()
		for iterator.Next() {
			if err := validateRuntimeValue(iterator.Key(), limits, depth+1, visited); err != nil { return err }
			if err := validateRuntimeValue(iterator.Value(), limits, depth+1, visited); err != nil { return err }
		}
	case reflect.Struct:
		if limits.MaxCollectionLen > 0 && uint64(value.NumField()) > limits.MaxCollectionLen {
			return &RuntimeLimitError{Limit: "MaxRecordFields", Maximum: limits.MaxCollectionLen, Actual: uint64(value.NumField()), Cause: ErrArgumentTooLarge}
		}
		for index := 0; index < value.NumField(); index++ {
			if value.Field(index).CanInterface() {
				if err := validateRuntimeValue(value.Field(index), limits, depth+1, visited); err != nil { return err }
			}
		}
	}
	return nil
}
