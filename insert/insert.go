package insert

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var errInvalidEntity = errors.New("invalid entity")

type SqlObject struct {
	table string
	cols  []string
	vals  []any
	seen  map[string]struct{}
}

func NewSqlStruct() *SqlObject {
	return &SqlObject{
		seen: make(map[string]struct{}),
	}
}

func (s *SqlObject) BuilderInsertSQL(entity any) error {
	val := reflect.ValueOf(entity)
	typ := val.Type()

	if !val.IsValid() {
		return errInvalidEntity
	}

	// 多级指针判断
	for i := 0; typ.Kind() == reflect.Ptr; i++ {
		if i > 0 {
			return errInvalidEntity
		}
		typ = typ.Elem()
		val = val.Elem()
	}

	// (*struct)(nil) or non-struct or empty struct
	if !val.IsValid() || val.Kind() != reflect.Struct || val.NumField() == 0 {
		return errInvalidEntity
	}

	if s.table == "" {
		s.setTable(typ.Name())
	}

	// driver.Valuer
	for i := 0; i < val.NumField(); i++ {
		fd := typ.Field(i)
		if s.existsColum(fd.Name) {
			continue
		}
		fdVal := val.Field(i)
		// isStruct := fdVal.Kind() == reflect.Struct
		isPtr := fd.Type.Kind() == reflect.Ptr
		isStruct := fd.Type.Kind() == reflect.Struct
		isAnonymousField := fd.Anonymous
		// 判断字段是否实现了 driver.Valuer 接口
		// valuer := (*driver.Valuer)(nil)
		var valuer *driver.Valuer = nil
		implementsInterface := fdVal.Type().Implements(
			reflect.TypeOf(valuer).Elem(),
		)

		if isStruct && isAnonymousField && isPtr && !implementsInterface {
			if err := s.BuilderInsertSQL(fd); err != nil {
				return err
			}

		}

		s.addCols(fd.Name, fdVal.Interface())
	}

	return nil
}

func (s *SqlObject) addCols(name string, val any) {
	col := "`" + name + "`"
	s.cols = append(s.cols, col)
	s.seen[col] = struct{}{}
	s.vals = append(s.vals, val)
}

func (s *SqlObject) existsColum(name string) bool {
	_, ok := s.seen["`"+name+"`"]
	return ok
}

func (s *SqlObject) values() []any {
	return s.vals
}

func (s *SqlObject) setTable(name string) {
	s.table = "`" + name + "`"
}

func (s *SqlObject) insertSqlString() string {
	if s.table == "" || s.cols == nil {
		return ""
	}
	query := fmt.Sprintf(
		"INSERT INTO %s(%s) VALUES(%s);",
		s.table, strings.Join(s.cols, ","),
		strings.TrimRight(strings.Repeat("?,", len(s.vals)), ","),
	)
	return query

}

func InsertStmt(entity interface{}) (string, []interface{}, error) {
	if entity == nil {
		return "", nil, errInvalidEntity
	}
	sql := NewSqlStruct()
	err := sql.BuilderInsertSQL(entity)
	if err != nil {
		return "", nil, err
	}
	return sql.insertSqlString(), sql.vals, nil
}
