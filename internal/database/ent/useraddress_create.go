// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
	"github.com/abisalde/authentication-service/internal/database/ent/useraddress"
)

// UserAddressCreate is the builder for creating a UserAddress entity.
type UserAddressCreate struct {
	config
	mutation *UserAddressMutation
	hooks    []Hook
}

// Mutation returns the UserAddressMutation object of the builder.
func (uac *UserAddressCreate) Mutation() *UserAddressMutation {
	return uac.mutation
}

// Save creates the UserAddress in the database.
func (uac *UserAddressCreate) Save(ctx context.Context) (*UserAddress, error) {
	return withHooks(ctx, uac.sqlSave, uac.mutation, uac.hooks)
}

// SaveX calls Save and panics if Save returns an error.
func (uac *UserAddressCreate) SaveX(ctx context.Context) *UserAddress {
	v, err := uac.Save(ctx)
	if err != nil {
		panic(err)
	}
	return v
}

// Exec executes the query.
func (uac *UserAddressCreate) Exec(ctx context.Context) error {
	_, err := uac.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (uac *UserAddressCreate) ExecX(ctx context.Context) {
	if err := uac.Exec(ctx); err != nil {
		panic(err)
	}
}

// check runs all checks and user-defined validators on the builder.
func (uac *UserAddressCreate) check() error {
	return nil
}

func (uac *UserAddressCreate) sqlSave(ctx context.Context) (*UserAddress, error) {
	if err := uac.check(); err != nil {
		return nil, err
	}
	_node, _spec := uac.createSpec()
	if err := sqlgraph.CreateNode(ctx, uac.driver, _spec); err != nil {
		if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return nil, err
	}
	id := _spec.ID.Value.(int64)
	_node.ID = int(id)
	uac.mutation.id = &_node.ID
	uac.mutation.done = true
	return _node, nil
}

func (uac *UserAddressCreate) createSpec() (*UserAddress, *sqlgraph.CreateSpec) {
	var (
		_node = &UserAddress{config: uac.config}
		_spec = sqlgraph.NewCreateSpec(useraddress.Table, sqlgraph.NewFieldSpec(useraddress.FieldID, field.TypeInt))
	)
	return _node, _spec
}

// UserAddressCreateBulk is the builder for creating many UserAddress entities in bulk.
type UserAddressCreateBulk struct {
	config
	err      error
	builders []*UserAddressCreate
}

// Save creates the UserAddress entities in the database.
func (uacb *UserAddressCreateBulk) Save(ctx context.Context) ([]*UserAddress, error) {
	if uacb.err != nil {
		return nil, uacb.err
	}
	specs := make([]*sqlgraph.CreateSpec, len(uacb.builders))
	nodes := make([]*UserAddress, len(uacb.builders))
	mutators := make([]Mutator, len(uacb.builders))
	for i := range uacb.builders {
		func(i int, root context.Context) {
			builder := uacb.builders[i]
			var mut Mutator = MutateFunc(func(ctx context.Context, m Mutation) (Value, error) {
				mutation, ok := m.(*UserAddressMutation)
				if !ok {
					return nil, fmt.Errorf("unexpected mutation type %T", m)
				}
				if err := builder.check(); err != nil {
					return nil, err
				}
				builder.mutation = mutation
				var err error
				nodes[i], specs[i] = builder.createSpec()
				if i < len(mutators)-1 {
					_, err = mutators[i+1].Mutate(root, uacb.builders[i+1].mutation)
				} else {
					spec := &sqlgraph.BatchCreateSpec{Nodes: specs}
					// Invoke the actual operation on the latest mutation in the chain.
					if err = sqlgraph.BatchCreate(ctx, uacb.driver, spec); err != nil {
						if sqlgraph.IsConstraintError(err) {
							err = &ConstraintError{msg: err.Error(), wrap: err}
						}
					}
				}
				if err != nil {
					return nil, err
				}
				mutation.id = &nodes[i].ID
				if specs[i].ID.Value != nil {
					id := specs[i].ID.Value.(int64)
					nodes[i].ID = int(id)
				}
				mutation.done = true
				return nodes[i], nil
			})
			for i := len(builder.hooks) - 1; i >= 0; i-- {
				mut = builder.hooks[i](mut)
			}
			mutators[i] = mut
		}(i, ctx)
	}
	if len(mutators) > 0 {
		if _, err := mutators[0].Mutate(ctx, uacb.builders[0].mutation); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

// SaveX is like Save, but panics if an error occurs.
func (uacb *UserAddressCreateBulk) SaveX(ctx context.Context) []*UserAddress {
	v, err := uacb.Save(ctx)
	if err != nil {
		panic(err)
	}
	return v
}

// Exec executes the query.
func (uacb *UserAddressCreateBulk) Exec(ctx context.Context) error {
	_, err := uacb.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (uacb *UserAddressCreateBulk) ExecX(ctx context.Context) {
	if err := uacb.Exec(ctx); err != nil {
		panic(err)
	}
}
