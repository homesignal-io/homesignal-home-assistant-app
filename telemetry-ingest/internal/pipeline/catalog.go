package pipeline

import (
	"fmt"
	"sort"
)

type Catalog struct {
	handlers map[SchemaKey]SchemaHandler
}

func NewDefaultCatalog() *Catalog {
	catalog := &Catalog{handlers: map[SchemaKey]SchemaHandler{}}
	catalog.Register(DeviceHealthSnapshotHandler{})
	catalog.Register(AgentAlarmHandler{})
	return catalog
}

func (c *Catalog) Register(handler SchemaHandler) {
	c.handlers[handler.Key()] = handler
}

func (c *Catalog) Validate(envelope RuntimeEnvelope) (Projection, error) {
	handler, ok := c.handlers[envelope.Key()]
	if !ok {
		return Projection{}, fmt.Errorf("unsupported runtime schema %s/%s/v%d", envelope.MessageType, envelope.SchemaType, envelope.SchemaVersion)
	}
	return handler.Validate(envelope)
}

func (c *Catalog) Supported() []SchemaKey {
	keys := make([]SchemaKey, 0, len(c.handlers))
	for key := range c.handlers {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].MessageType != keys[j].MessageType {
			return keys[i].MessageType < keys[j].MessageType
		}
		if keys[i].SchemaType != keys[j].SchemaType {
			return keys[i].SchemaType < keys[j].SchemaType
		}
		return keys[i].Version < keys[j].Version
	})
	return keys
}
