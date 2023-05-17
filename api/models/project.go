package models

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/lib/pq"
	"time"
)

type Project struct {
	Name    string         `json:"name" bson:"name" valid:"required~please provide a valid name"`
	Type    string         `json:"type" bson:"type" valid:"required~please provide a valid type,in(incoming|outgoing)"`
	LogoURL string         `json:"logo_url" bson:"logo_url" valid:"url~please provide a valid logo url,optional"`
	Config  *ProjectConfig `json:"config"`
}

type UpdateProject struct {
	Name              string         `json:"name" bson:"name" valid:"required~please provide a valid name"`
	LogoURL           string         `json:"logo_url" bson:"logo_url" valid:"url~please provide a valid logo url,optional"`
	RateLimit         int            `json:"rate_limit" bson:"rate_limit" valid:"int~please provide a valid rate limit,optional"`
	RateLimitDuration string         `json:"rate_limit_duration" bson:"rate_limit_duration" valid:"alphanum~please provide a valid rate limit duration,optional"`
	Config            *ProjectConfig `json:"config" valid:"optional"`
}

type ProjectConfig struct {
	MaxIngestSize            uint64                        `json:"max_payload_read_size"`
	ReplayAttacks            bool                          `json:"replay_attacks_prevention_enabled"`
	IsRetentionPolicyEnabled bool                          `json:"retention_policy_enabled"`
	DisableEndpoint          bool                          `json:"disable_endpoint"`
	RetentionPolicy          *RetentionPolicyConfiguration `json:"retention_policy"`
	RateLimit                *RateLimitConfiguration       `json:"ratelimit"`
	Strategy                 *StrategyConfiguration        `json:"strategy"`
	Signature                *SignatureConfiguration       `json:"signature"`
	MetaEvent                *MetaEventConfiguration       `json:"meta_event"`
}

func (pc *ProjectConfig) Convert() *datastore.ProjectConfig {
	if pc == nil {
		return nil
	}

	return &datastore.ProjectConfig{
		MaxIngestSize:            pc.MaxIngestSize,
		ReplayAttacks:            pc.ReplayAttacks,
		IsRetentionPolicyEnabled: pc.IsRetentionPolicyEnabled,
		DisableEndpoint:          pc.DisableEndpoint,
		RetentionPolicy:          pc.RetentionPolicy.convert(),
		RateLimit:                pc.RateLimit.convert(),
		Strategy:                 pc.Strategy.convert(),
		Signature:                pc.Signature.convert(),
		MetaEvent:                pc.MetaEvent.convert(),
	}
}

type RetentionPolicyConfiguration struct {
	Policy string `json:"policy" valid:"required~please provide a valid retention policy"`
}

func (r *RetentionPolicyConfiguration) convert() *datastore.RetentionPolicyConfiguration {
	if r == nil {
		return nil
	}

	return &datastore.RetentionPolicyConfiguration{Policy: r.Policy}
}

type RateLimitConfiguration struct {
	Count    int    `json:"count"`
	Duration uint64 `json:"duration"`
}

func (rc *RateLimitConfiguration) convert() *datastore.RateLimitConfiguration {
	if rc == nil {
		return nil
	}

	return &datastore.RateLimitConfiguration{Count: rc.Count, Duration: rc.Duration}
}

type StrategyConfiguration struct {
	Type       string `json:"type" valid:"optional~please provide a valid strategy type, in(linear|exponential)~unsupported strategy type"`
	Duration   uint64 `json:"duration" valid:"optional~please provide a valid duration in seconds,int"`
	RetryCount uint64 `json:"retry_count" valid:"optional~please provide a valid retry count,int"`
}

func (sc *StrategyConfiguration) convert() *datastore.StrategyConfiguration {
	if sc == nil {
		return nil
	}

	return &datastore.StrategyConfiguration{
		Type:       datastore.StrategyProvider(sc.Type),
		Duration:   sc.Duration,
		RetryCount: sc.RetryCount,
	}
}

type SignatureConfiguration struct {
	Hash     string                         `json:"-"` // Deprecated
	Header   config.SignatureHeaderProvider `json:"header,omitempty" valid:"required~please provide a valid signature header"`
	Versions []SignatureVersion             `json:"versions"`
}

func (sc *SignatureConfiguration) convert() *datastore.SignatureConfiguration {
	if sc == nil {
		return nil
	}

	s := &datastore.SignatureConfiguration{Header: sc.Header, Hash: sc.Hash}
	for _, version := range sc.Versions {
		s.Versions = append(s.Versions, datastore.SignatureVersion{
			UID:       version.UID,
			Hash:      version.Hash,
			Encoding:  datastore.EncodingType(version.Encoding),
			CreatedAt: version.CreatedAt,
		})
	}

	return s
}

type SignatureVersion struct {
	UID       string    `json:"uid" db:"id"`
	Hash      string    `json:"hash,omitempty" valid:"required~please provide a valid hash,supported_hash~unsupported hash type"`
	Encoding  string    `json:"encoding" valid:"required~please provide a valid signature header"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type MetaEventConfiguration struct {
	IsEnabled bool           `json:"is_enabled"`
	Type      string         `json:"type" valid:"optional, in(http)~unsupported meta event type"`
	EventType pq.StringArray `json:"event_type"`
	URL       string         `json:"url"`
	Secret    string         `json:"secret"`
}

func (mc *MetaEventConfiguration) convert() *datastore.MetaEventConfiguration {
	if mc == nil {
		return nil
	}

	return &datastore.MetaEventConfiguration{
		IsEnabled: mc.IsEnabled,
		Type:      datastore.MetaEventType(mc.Type),
		EventType: mc.EventType,
		URL:       mc.URL,
		Secret:    mc.Secret,
	}
}
