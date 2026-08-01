package witgo

import (
	"encoding/json"
	"reflect"
)

const hardMaxRuntimeValueDepth uint32 = 256

type ValueLimits struct {
	MaxArgumentBytes uint64
	MaxResultBytes   uint64
	MaxCollectionLen uint64
	MaxValueDepth    uint32
	MaxStringBytes   uint64
}

func validateArguments(values []any, limits ValueLimits) error {
	return validateValues(values, limits, limits.MaxArgumentBytes, "MaxArgumentBytes", ErrArgumentTooLarge)
}

func validateResults(values []any, limits ValueLimits) error {
	return validateValues(values, limits, limits.MaxResultBytes, "MaxResultBytes", ErrResultTooLarge)
}

func validateValues(values []any, limits ValueLimits, maxBytes uint64, byteLimitName string, byteLimitCause error) error {
	// Даже при отключённом пользовательском лимите оставляем внутренний
	// предохранитель: циклическая map/interface-структура не должна переполнить
	// стек процесса хоста.
	if limits.MaxValueDepth == 0 || limits.MaxValueDepth > hardMaxRuntimeValueDepth {
		limits.MaxValueDepth = hardMaxRuntimeValueDepth
	}
	visited := make(map[valueVisit]bool)
	for _, value := range values {
		if err := validateRuntimeValue(reflect.ValueOf(value), limits, 1, visited); err != nil {
			return err
		}
	}
	if maxBytes > 0 {
		data, err := json.Marshal(values)
		if err != nil {
			return err
		}
		if uint64(len(data)) > maxBytes {
			return &RuntimeLimitError{Limit: byteLimitName, Maximum: maxBytes, Actual: uint64(len(data)), Cause: byteLimitCause}
		}
	}
	return nil
}

type valueVisit struct {
	typ     reflect.Type
	pointer uintptr
}

func validateRuntimeValue(value reflect.Value, limits ValueLimits, depth uint32, visited map[valueVisit]bool) error {
	if !value.IsValid() {
		return nil
	}
	if limits.MaxValueDepth > 0 && depth > limits.MaxValueDepth {
		return &RuntimeLimitError{Limit: "MaxValueDepth", Maximum: uint64(limits.MaxValueDepth), Actual: uint64(depth), Cause: ErrValueDepthExceeded}
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		if value.Kind() == reflect.Pointer {
			visit := valueVisit{typ: value.Type(), pointer: value.Pointer()}
			if visited[visit] {
				return &RuntimeLimitError{Limit: "MaxValueDepth", Maximum: uint64(limits.MaxValueDepth), Actual: uint64(depth + 1), Cause: ErrValueDepthExceeded}
			}
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
			if visited[visit] {
				return &RuntimeLimitError{Limit: "MaxValueDepth", Maximum: uint64(limits.MaxValueDepth), Actual: uint64(depth + 1), Cause: ErrValueDepthExceeded}
			}
			visited[visit] = true
			defer delete(visited, visit)
		}
		for index := 0; index < value.Len(); index++ {
			if err := validateRuntimeValue(value.Index(index), limits, depth+1, visited); err != nil {
				return err
			}
		}
	case reflect.Map:
		if value.IsNil() {
			return nil
		}
		if limits.MaxCollectionLen > 0 && uint64(value.Len()) > limits.MaxCollectionLen {
			return &RuntimeLimitError{Limit: "MaxCollectionLen", Maximum: limits.MaxCollectionLen, Actual: uint64(value.Len()), Cause: ErrArgumentTooLarge}
		}
		// reflect не предоставляет переносимый identity для map. Цикл всё равно
		// безопасно останавливает обязательный внутренний лимит глубины выше.
		iterator := value.MapRange()
		for iterator.Next() {
			if err := validateRuntimeValue(iterator.Key(), limits, depth+1, visited); err != nil {
				return err
			}
			if err := validateRuntimeValue(iterator.Value(), limits, depth+1, visited); err != nil {
				return err
			}
		}
	case reflect.Struct:
		if limits.MaxCollectionLen > 0 && uint64(value.NumField()) > limits.MaxCollectionLen {
			return &RuntimeLimitError{Limit: "MaxRecordFields", Maximum: limits.MaxCollectionLen, Actual: uint64(value.NumField()), Cause: ErrArgumentTooLarge}
		}
		for index := 0; index < value.NumField(); index++ {
			if value.Field(index).CanInterface() {
				if err := validateRuntimeValue(value.Field(index), limits, depth+1, visited); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
