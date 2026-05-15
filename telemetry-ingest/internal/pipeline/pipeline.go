package pipeline

import (
	"context"
	"fmt"
	"time"
)

type RuntimePipeline struct {
	Parser      EnvelopeParser
	Catalog     *Catalog
	Writer      PersistenceWriter
	FailureSink FailureSink
	Clock       func() time.Time
}

func NewRuntimePipeline(writer PersistenceWriter, failureSink FailureSink) *RuntimePipeline {
	return &RuntimePipeline{
		Parser:      RuntimeEnvelopeParser{},
		Catalog:     NewDefaultCatalog(),
		Writer:      writer,
		FailureSink: failureSink,
		Clock:       func() time.Time { return time.Now().UTC() },
	}
}

func (p *RuntimePipeline) Ingest(ctx context.Context, request IngestRequest) (IngestResult, error) {
	p = p.withDefaults()
	receivedAt := request.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = p.Clock().UTC()
	}

	envelope, err := p.Parser.Parse(request.Route, request.Body)
	if err != nil {
		p.recordFailure(ctx, request.Device, RuntimeEnvelope{}, "parse", err.Error(), receivedAt)
		return IngestResult{Accepted: false, ReceivedAt: receivedAt}, err
	}

	projection, err := p.Catalog.Validate(envelope)
	if err != nil {
		p.recordFailure(ctx, request.Device, envelope, "schema", err.Error(), receivedAt)
		return IngestResult{Accepted: false, MessageID: envelope.MessageID, SchemaType: envelope.SchemaType, ReceivedAt: receivedAt}, err
	}

	message := ValidatedMessage{
		Device:     request.Device,
		Envelope:   envelope,
		Projection: projection,
		ReceivedAt: receivedAt,
	}
	if p.Writer != nil {
		if err := p.Writer.WriteLatest(ctx, message); err != nil {
			p.recordFailure(ctx, request.Device, envelope, "persistence", err.Error(), receivedAt)
			return IngestResult{Accepted: false, MessageID: envelope.MessageID, SchemaType: envelope.SchemaType, ReceivedAt: receivedAt}, fmt.Errorf("write latest state: %w", err)
		}
	}

	return IngestResult{
		Accepted:   true,
		MessageID:  envelope.MessageID,
		SchemaType: envelope.SchemaType,
		ReceivedAt: receivedAt,
	}, nil
}

func (p *RuntimePipeline) withDefaults() *RuntimePipeline {
	if p == nil {
		return NewRuntimePipeline(nil, nil)
	}
	if p.Parser == nil {
		p.Parser = RuntimeEnvelopeParser{}
	}
	if p.Catalog == nil {
		p.Catalog = NewDefaultCatalog()
	}
	if p.Clock == nil {
		p.Clock = func() time.Time { return time.Now().UTC() }
	}
	return p
}

func (p *RuntimePipeline) recordFailure(ctx context.Context, device AuthenticatedDeviceContext, envelope RuntimeEnvelope, stage, reason string, receivedAt time.Time) {
	if p.FailureSink == nil {
		return
	}
	_ = p.FailureSink.RecordFailure(ctx, IngestFailure{
		Device:     device,
		MessageID:  envelope.MessageID,
		SchemaType: envelope.SchemaType,
		Stage:      stage,
		Reason:     reason,
		ReceivedAt: receivedAt,
	})
}
