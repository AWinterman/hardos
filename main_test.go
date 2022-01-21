package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

type mocks int

func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name, args.Inputs, nil
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func Test_nodes(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "creates nodes labelled with index",
			args:    args{},
			wantErr: false,
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				nodes, err := nodes(ctx)
				assert.NoError(t, err)

				var wg sync.WaitGroup
				wg.Add(1)

				pulumi.All(nodes[0].ID()).ApplyT(func(all []interface{}) error {
					id := all[0].(pulumi.ID)
					assert.Equal(t, pulumi.ID("node-0"), id, "ids match")
					wg.Done()
					return nil
				})

				wg.Wait()
				return nil
			}, pulumi.WithMocks("project", "stack", mocks(0)))
			assert.NoError(t, err)
		})
	}
}
