package form

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fm struct {
	Name string
	Birthdate time.Time
}

func (x *fm) MarshalForm() ([]byte, error) {
	s := fmt.Sprintf("n=%s&b=%s", url.QueryEscape(x.Name), url.QueryEscape(x.Birthdate.In(time.UTC).Format("2006-01-02")))
	return []byte(s), nil
}

func (x *fm) UnmarshalForm(data []byte) error {
	query, err := url.ParseQuery(string(data))
	if err != nil {
		return err
	}
	t, err := time.ParseInLocation("2006-01-02", query.Get("b"), time.UTC)
	if err != nil {
		return err
	}
	x.Name = query.Get("n")
	x.Birthdate = t
	return nil
}

func TestFormMarshaler(t *testing.T) {
	x := &fm{Name: "John", Birthdate: time.Date(1940, time.October, 9, 0, 0, 0, 0, time.UTC)}
	data, err := MarshalForm(x)
	assert.Nil(t, err)
	assert.Equal(t, "n=John&b=1940-10-09", string(data))
}

func TestFormUnmarshaler(t *testing.T) {
	data := []byte("n=John&b=1940-10-09")
	x := &fm{}
	err := UnmarshalForm(data, x)
	assert.Nil(t, err)
	assert.Equal(t, "John", x.Name)
	assert.Equal(t, 1940, x.Birthdate.Year())
	assert.Equal(t, time.October, x.Birthdate.Month())
	assert.Equal(t, 9, x.Birthdate.Day())
}

type testStruct struct {
	Name []string `json:"name"`
	Birthdate time.Time `json:"birth"`
	Age float64 `json:"age"`
	FavoriteNumbers []int `json:"numbers"`
}

func TestMarshal(t *testing.T) {
	x := &testStruct{
		Name: []string{"John", "Lennon"},
		Birthdate: time.Date(1940, time.October, 9, 0, 0, 0, 0, time.UTC),
		Age: 81.8,
		FavoriteNumbers: []int{5, 7},
	}
	data, err := MarshalForm(x)
	assert.Nil(t, err)
	assert.Equal(t, "name=John&name=Lennon&birth=1940-10-09T00%3A00%3A00Z&age=81.8&numbers=5&numbers=7", string(data))
}

func TestUnmarshal(t *testing.T) {
	data := []byte("name=John&name=Lennon&birth=1940-10-09T00%3A00%3A00Z&age=81.8&numbers=5&numbers=7")
	x := &testStruct{}
	err := UnmarshalForm(data, x)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(x.Name))
	assert.Equal(t, "John", x.Name[0])
	assert.Equal(t, "Lennon", x.Name[1])
	assert.Equal(t, 1940, x.Birthdate.Year())
	assert.Equal(t, time.October, x.Birthdate.Month())
	assert.Equal(t, 9, x.Birthdate.Day())
	assert.Equal(t, 81.8, x.Age)
	assert.Equal(t, 2, len(x.FavoriteNumbers))
	assert.Equal(t, 5, x.FavoriteNumbers[0])
	assert.Equal(t, 7, x.FavoriteNumbers[1])

	m := map[string]interface{}{}
	err = UnmarshalForm(data, &m)
	assert.Nil(t, err)
	ss, ok := m["name"].([]string)
	assert.True(t, ok, "name is []string")
	assert.Equal(t, 2, len(ss))
	assert.Equal(t, "John", ss[0])
	assert.Equal(t, "Lennon", ss[1])
	tm, ok := m["birth"].(time.Time)
	assert.True(t, ok, "birth is time.Time")
	assert.Equal(t, 1940, tm.Year())
	assert.Equal(t, time.October, tm.Month())
	assert.Equal(t, 9, tm.Day())
	f, ok := m["age"].(float64)
	assert.True(t, ok, "age is float64")
	assert.Equal(t, 81.8, f)
	ints, ok := m["numbers"].([]int64)
	assert.True(t, ok, "numbers is []int64")
	assert.Equal(t, 2, len(ints))
	assert.Equal(t, int64(5), ints[0])
	assert.Equal(t, int64(7), ints[1])
}
