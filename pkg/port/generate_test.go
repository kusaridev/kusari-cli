package port

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_generateRandomPort(t *testing.T) {
	actualStr, _ := _generateRandomPort()
	actual, e := strconv.Atoi(actualStr)

	assert.Nil(t, e)
	assert.True(t, actual >= 62001)
	assert.True(t, actual <= 62009)
}
func Test_GenerateRandomPortOrDefault_Default(t *testing.T) {
	mock := &Mock{port: "", err: fmt.Errorf("Jolene, Jolene, Jolene, Don't Leave")}
	generateRandomPort = mock.generateRandomPort

	fmt.Print("Mock test run")
	actual := GenerateRandomPortOrDefault()

	assert.Equal(t, "62009", actual)
}

func Test_GenerateRandomPortOrDefault_1234(t *testing.T) {
	mock := &Mock{port: "1234", err: nil}
	generateRandomPort = mock.generateRandomPort

	fmt.Print("Mock test run")
	actual := GenerateRandomPortOrDefault()

	assert.Equal(t, "1234", actual)
}

type Mock struct {
	port string
	err  error
}

func (m *Mock) generateRandomPort() (string, error) {
	return m.port, m.err
}
