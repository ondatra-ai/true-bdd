package scenarios

import (
	"fixture/stepsrun/tests/calc/steps"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"testing"
 "os"
)

func New(t *testing.T, id string) *bddgo.Run[steps.State] {
	t.Helper()
 if _,err:=os.Stat("../../services/calc");err!=nil{t.Fatalf("bring the suite up: %v",err)}
	suite, err := bddgo.New[steps.State](bddgo.Options{Registry: "../../docs/scenarios.yaml", Architecture: "../../docs/architecture/architecture.yaml", Suite: "calc"})
	if err != nil {
		t.Fatal(err)
	}
	suite.Init(func(_ *bddgo.World) (*steps.State, error) { return &steps.State{}, nil })
	steps.Register(suite)
	return suite.Scenario(t, id)
}
