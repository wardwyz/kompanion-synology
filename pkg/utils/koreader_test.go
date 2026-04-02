package utils_test

import (
	"testing"

	"github.com/vanadium23/kompanion/pkg/utils"
)

func TestPartialMd5(t *testing.T) {
	expected := "5ee88058c4346a122c4ccf80e36b1dc8"
	actual, err := utils.PartialMD5("../../test/test_data/books/CrimePunishment-EPUB2.epub")
	if err != nil {
		t.Fatalf("Error calculating MD5: %v", err)
	}
	if expected != actual {
		t.Fatalf("Expected MD5 %s, got %s", expected, actual)
	}
}
