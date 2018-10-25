package main

import (
	"reflect"
	"testing"
)

func TestMatchedKeyFromDict(t *testing.T) {
	dict := map[string]Pattern{"foo": Pattern{}, "foo bar": Pattern{}}
	k, _ := matchedKeyFromDict(dict, []string{"foo", "bar"})
	expected := "foo bar"
	if !reflect.DeepEqual(expected, k) {
		t.Errorf("Return value should be %v, but received %v",
			expected, k)
	}
}
func TestMatchedKeyFromDict_EmptyKey(t *testing.T) {
	dict := map[string]Pattern{"": Pattern{}, "foo": Pattern{}}
	k, _ := matchedKeyFromDict(dict, []string{"bar", "baz"})
	expected := ""
	if !reflect.DeepEqual(expected, k) {
		t.Errorf("Return value should be %v, but received %v",
			expected, k)
	}
}
func TestMatchedKeyFromDict_NeverMatch(t *testing.T) {
	dict := map[string]Pattern{"foo bar": Pattern{}, "foo": Pattern{}}
	k, err := matchedKeyFromDict(dict, []string{"bar", "baz"})
	if err == nil {
		t.Errorf("Error should be returned, but received %v", k)
	}
}
