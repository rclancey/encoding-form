package form

import (
	"encoding"
	"errors"
	"fmt"
	"log"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type FormMarshaler interface {
	MarshalForm() ([]byte, error)
}

type FormUnmarshaler interface {
	UnmarshalForm([]byte) error
}

func MarshalForm(obj interface{}) ([]byte, error) {
	switch x := obj.(type) {
	case FormMarshaler:
		return x.MarshalForm()
	case url.Values:
		return []byte(x.Encode()), nil
	case map[string]string:
		values := url.Values{}
		for k, v := range x {
			values.Set(k, v)
		}
		return []byte(values.Encode()), nil
	case map[string][]string:
		return MarshalForm(url.Values(x))
	case string:
		return []byte(x), nil
	case []byte:
		return x, nil
	}
	rv := reflect.ValueOf(obj)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Struct {
		rt := rv.Type()
		n := rt.NumField()
		pairs := make([]string, 0, n)
		for i := 0; i < n; i++ {
			rf := rt.Field(i)
			if rf.PkgPath != "" {
				continue
			}
			tag := strings.Split(rf.Tag.Get("json"), ",")[0]
			if tag == "-" {
				continue
			}
			if tag == "" {
				tag = strings.ToLower(rf.Name)
			}
			val := rv.Field(i)
			if val.Kind() == reflect.Ptr {
				if val.IsNil() {
					continue
				}
				val = val.Elem()
			}
			if val.Kind() == reflect.Slice {
				for j := 0; j < val.Len(); j++ {
					pair := fmt.Sprintf("%s=%s", url.QueryEscape(tag), url.QueryEscape(asString(val.Index(j))))
					pairs = append(pairs, pair)
				}
			} else {
				pair := fmt.Sprintf("%s=%s", url.QueryEscape(tag), url.QueryEscape(asString(val)))
				pairs = append(pairs, pair)
			}
		}
		return []byte(strings.Join(pairs, "&")), nil
	}
	if rv.Kind() == reflect.Map {
		values := url.Values{}
		iter := rv.MapRange()
		for iter.Next() {
			values.Set(asString(iter.Key()), asString(iter.Value()))
		}
		return []byte(values.Encode()), nil
	}
	return []byte(asString(rv)), nil
}

func pascalParts(s string) []string {
	parts := []string{}
	start := 0
	n := len(s)
	i := 0
	bs := []byte(s)
	for i < n {
		r, j := utf8.DecodeRune(bs[i:])
		if i > 0 && unicode.IsUpper(r) {
			parts = append(parts, strings.ToLower(s[start:i]))
			start = i
		}
		i += j
	}
	parts = append(parts, s[start:n])
	return parts
}

func camelCase(s string) string {
	return strings.ToLower(s[:1]) + s[1:]
}

func snakeCase(parts []string) string {
	return strings.Join(parts, "_")
}

func kebabCase(parts []string) string {
	return strings.Join(parts, "-")
}

func UnmarshalForm(data []byte, obj interface{}) error {
	switch tobj := obj.(type) {
	case FormUnmarshaler:
		return tobj.UnmarshalForm(data)
	case *url.Values:
		query, err := url.ParseQuery(string(data))
		if err != nil {
			return err
		}
		*tobj = query
		return nil
	}
	rv := reflect.ValueOf(obj)
	if rv.Kind() != reflect.Ptr {
		return errors.New("not a pointer")
	}
	query, err := url.ParseQuery(string(data))
	if err != nil {
		return err
	}
	rv = rv.Elem()
	rt := rv.Type()
	switch rv.Kind() {
	case reflect.Struct:
		keys := map[string]int{}
		n := rt.NumField()
		for i := 0; i < n; i++ {
			rf := rt.Field(i)
			if rf.PkgPath != "" {
				continue
			}
			keys[rf.Name] = i
			keys[strings.ToLower(rf.Name)] = i
			keys[camelCase(rf.Name)] = i
			parts := pascalParts(rf.Name)
			keys[snakeCase(parts)] = i
			keys[kebabCase(parts)] = i
		}
		for i := 0; i < n; i++ {
			rf := rt.Field(i)
			if rf.PkgPath != "" {
				continue
			}
			tag := strings.Split(rf.Tag.Get("json"), ",")[0]
			if tag != "" {
				keys[tag] = i
			}
		}
		for k, vals := range query {
			i, ok := keys[k]
			if !ok {
				continue
			}
			v := reflect.New(rt.Field(i).Type)
			err := fromStrings(vals, v.Interface())
			if err != nil {
				return err
			}
			rv.Field(i).Set(v.Elem())
		}
	case reflect.Map:
		for key, vals := range query {
			kv := reflect.New(rt.Key())
			err := fromString(key, kv.Interface())
			if err != nil {
				return err
			}
			pv := reflect.New(rt.Elem())
			err = fromStrings(vals, pv.Interface())
			if err != nil {
				return err
			}
			rv.SetMapIndex(kv.Elem(), pv.Elem())
		}
	default:
		return fmt.Errorf("can't unmarshal to %T", obj)
	}
	return nil
}

func asString(val reflect.Value) string {
	switch val.Kind() {
	case reflect.String:
		return val.String()
	case reflect.Bool:
		return strconv.FormatBool(val.Bool())
	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
		return strconv.FormatInt(val.Int(), 10)
	case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
		return strconv.FormatUint(val.Uint(), 10)
	case reflect.Float64, reflect.Float32:
		return strconv.FormatFloat(val.Float(), 'f', -1, 64)
	}
	ival := val.Interface()
	tval, ok := ival.(encoding.TextMarshaler)
	if ok {
		text, err := tval.MarshalText()
		if err == nil {
			return string(text)
		}
	}
	sval, ok := ival.(fmt.Stringer)
	if ok {
		return sval.String()
	}
	return fmt.Sprintf("%#v", ival)
}

var layouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05Z07:00",
	time.RFC822Z,
	time.RFC822,
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func fromString(val string, obj interface{}) error {
	tum, ok := obj.(encoding.TextUnmarshaler)
	if ok {
		return tum.UnmarshalText([]byte(val))
	}
	bytesptr, ok := obj.(*[]byte)
	if ok {
		*bytesptr = []byte(val)
		return nil
	}
	rv := reflect.ValueOf(obj).Elem()
	switch rv.Kind() {
	case reflect.Interface:
		i, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			rv.Set(reflect.ValueOf(i))
			return nil
		}
		f, err := strconv.ParseFloat(val, 64)
		if err == nil {
			rv.Set(reflect.ValueOf(f))
			return nil
		}
		b, err := strconv.ParseBool(val)
		if err == nil {
			rv.Set(reflect.ValueOf(b))
			return nil
		}
		dur, err := time.ParseDuration(val)
		if err == nil {
			rv.Set(reflect.ValueOf(dur))
			return nil
		}
		for _, layout := range layouts {
			t, err := time.ParseInLocation(layout, val, time.UTC)
			if err == nil {
				rv.Set(reflect.ValueOf(t))
				return nil
			}
		}
		rv.Set(reflect.ValueOf(val))
		return nil
	case reflect.String:
		rv.SetString(val)
		return nil
	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
		i, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		rv.SetInt(i)
		return nil
	case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
		u, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return err
		}
		rv.SetUint(u)
		return nil
	case reflect.Float64, reflect.Float32:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
		rv.SetFloat(f)
		return nil
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		rv.SetBool(b)
		return nil
	}
	log.Panicf("can't parse (%s) into %T (%s)", val, obj, rv.Kind())
	return fmt.Errorf("can't parse (%s) into %T (%s)", val, obj, rv.Kind())
}

func fromStrings(vals []string, obj interface{}) error {
	rv := reflect.ValueOf(obj).Elem()
	switch rv.Kind() {
	case reflect.Interface:
		if len(vals) == 1 {
			pv := reflect.New(rv.Type())
			err := fromString(vals[0], pv.Interface())
			if err != nil {
				return err
			}
			rv.Set(pv.Elem())
		} else {
			pv := reflect.MakeSlice(reflect.SliceOf(rv.Type()), len(vals), len(vals))
			var stype reflect.Type
			stypes := 0
			for i, v := range vals {
				iv := reflect.New(rv.Type())
				err := fromString(v, iv.Interface())
				if err != nil {
					return err
				}
				if i == 0 {
					stype = reflect.ValueOf(iv.Elem().Interface()).Type()
					stypes += 1
				} else if stypes == 1 {
					if stype != reflect.ValueOf(iv.Elem().Interface()).Type() {
						stypes += 1
					}
				}
				pv.Index(i).Set(iv.Elem())
			}
			if stypes == 1 {
				sv := reflect.MakeSlice(reflect.SliceOf(stype), len(vals), len(vals))
				for i := 0; i < len(vals); i++ {
					sv.Index(i).Set(reflect.ValueOf(pv.Index(i).Interface()))
				}
				rv.Set(sv)
			} else {
				rv.Set(pv)
			}
		}
		return nil
	case reflect.Slice:
		pv := reflect.MakeSlice(rv.Type(), len(vals), len(vals))
		for i, v := range vals {
			iv := reflect.New(rv.Type().Elem())
			err := fromString(v, iv.Interface())
			if err != nil {
				return err
			}
			pv.Index(i).Set(iv.Elem())
		}
		rv.Set(pv)
		return nil
	case reflect.String:
		if len(vals) == 0 {
			rv.SetString("")
		} else if len(vals) == 1 {
			rv.SetString(vals[0])
		} else {
			rv.SetString(strings.Join(vals, ","))
		}
		return nil
	}
	if len(vals) == 0 {
		return nil
	}
	return fromString(vals[len(vals)-1], obj)
}
