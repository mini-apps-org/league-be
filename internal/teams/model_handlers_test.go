package teams

import (
	"reflect"
	"testing"
	"time"
)

// The response mappers copy by field name, so a field renamed on only one side compiles
// and is silently served as its zero value. Filling every source field and checking every
// response field is set catches that before a client sees an empty member_id.
func TestResponsesCopyEveryField(t *testing.T) {
	tests := []struct {
		name string
		run  func() any
	}{
		{"TeamResponse", func() any {
			var team Team
			fillEveryField(t, &team)
			return newTeamResponse(team)
		}},
		{"DivisionTeamResponse", func() any {
			var entry DivisionTeam
			fillEveryField(t, &entry)
			return newDivisionTeamResponse(entry)
		}},
		{"RosterPlayerResponse", func() any {
			var player RosterPlayer
			fillEveryField(t, &player)
			return newRosterPlayerResponse(player)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := reflect.ValueOf(tt.run())
			for i := range response.NumField() {
				if response.Field(i).IsZero() {
					t.Errorf("%s.%s was not filled: no source field of that name and type",
						response.Type().Name(), response.Type().Field(i).Name)
				}
			}
		})
	}
}

// fillEveryField gives each field of the struct target points at a non-zero value.
func fillEveryField(t *testing.T, target any) {
	t.Helper()
	value := reflect.ValueOf(target).Elem()
	for i := range value.NumField() {
		field := value.Field(i)
		switch field.Interface().(type) {
		case string:
			field.SetString("x")
		case *string:
			text := "x"
			field.Set(reflect.ValueOf(&text))
		case bool:
			field.SetBool(true)
		case int16, int32, int64:
			field.SetInt(1)
		case time.Time:
			field.Set(reflect.ValueOf(time.Unix(1, 0)))
		default:
			t.Fatalf("fillEveryField: add a case for %s.%s (%s)",
				value.Type().Name(), value.Type().Field(i).Name, field.Type())
		}
	}
}
