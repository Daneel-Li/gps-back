package utils

import "reflect"

func RemoveGormModelFields(updates map[string]interface{}) {
	// gorm.Model 的固定字段名
	protectedFields := []string{"CreatedAt", "UpdatedAt", "DeletedAt"}

	for _, field := range protectedFields {
		delete(updates, field)
	}
}

func StructToUpdateMap(s interface{}) map[string]interface{} {
	t := reflect.TypeOf(s)
	v := reflect.ValueOf(s)
	var data = make(map[string]interface{})
	//只取不是空指针的字段
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type.Kind() == reflect.Ptr {
			if v.Field(i).IsNil() {
				continue
			}
		}
		data[f.Name] = v.Field(i).Interface()
	}
	return data
}
