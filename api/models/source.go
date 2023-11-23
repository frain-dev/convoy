package models

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
)

type CreateSource struct {
	Name            string                   `json:"name" valid:"required~please provide a source name"`
	Type            datastore.SourceType     `json:"type" valid:"required~please provide a type,supported_source~unsupported source type"`
	Provider        datastore.SourceProvider `json:"provider"`
	IsDisabled      bool                     `json:"is_disabled"`
	CustomResponse  CustomResponse           `json:"custom_response"`
	Verifier        VerifierConfig           `json:"verifier"`
	PubSub          PubSubConfig             `json:"pub_sub"`
	IdempotencyKeys []string                 `json:"idempotency_keys"`
	IdempotencyTTL  string                   `json:"idempotency_ttl"`
}

func (cs *CreateSource) Validate() error {
	if cs.Provider.IsValid() {
		if err := validateSourceForProvider(cs); err != nil {
			return err
		}
	}

	if err := util.Validate(cs); err != nil {
		return err
	}

	if err := validateSourceVerifier(cs.Verifier); err != nil {
		return err
	}

	if err := validateIdempotencyKeyFormat(cs.IdempotencyKeys); err != nil {
		return err
	}

	return nil
}

func validateSourceVerifier(cfg VerifierConfig) error {
	if cfg.Type == datastore.HMacVerifier && cfg.HMac == nil {
		return errors.New("invalid verifier config for hmac")
	}

	if cfg.Type == datastore.APIKeyVerifier && cfg.ApiKey == nil {
		return errors.New("invalid verifier config for api key")
	}

	if cfg.Type == datastore.BasicAuthVerifier && cfg.BasicAuth == nil {
		return errors.New("invalid verifier config for basic auth")
	}

	return nil
}

func validateIdempotencyKeyFormat(input []string) error {
	for _, s := range input {
		parts := strings.Split(s, ".")
		if len(parts) < 3 {
			return fmt.Errorf("not enough parts set for idempotency key location with value: %s", s)
		}

		switch parts[0] {
		case "request", "req":
			switch parts[1] {
			case "Header", "header", "Body", "body", "QueryParam", "query":
				continue
			default:
				return fmt.Errorf("unsupported input format for idempotency key location with value: %s", s)
			}
		default:
			return fmt.Errorf("unsupported input format for idempotency key location with value: %s", s)
		}
	}

	return nil
}

func validateSourceForProvider(newSource *CreateSource) error {
	if util.IsStringEmpty(newSource.Name) {
		return errors.New("please provide a source name")
	}

	if !newSource.Type.IsValid() {
		return errors.New("please provide a valid source type")
	}

	switch newSource.Provider {
	case datastore.GithubSourceProvider,
		datastore.ShopifySourceProvider,
		datastore.TwitterSourceProvider:
		verifierConfig := newSource.Verifier
		if verifierConfig.HMac == nil || verifierConfig.HMac.Secret == "" {
			return fmt.Errorf("hmac secret is required for %s source", newSource.Provider)
		}
	}

	return nil
}

type UpdateSource struct {
	Name            *string              `json:"name" valid:"required~please provide a source name"`
	Type            datastore.SourceType `json:"type" valid:"required~please provide a type,supported_source~unsupported source type"`
	IsDisabled      *bool                `json:"is_disabled"`
	ForwardHeaders  []string             `json:"forward_headers"`
	CustomResponse  UpdateCustomResponse `json:"custom_response"`
	Verifier        VerifierConfig       `json:"verifier"`
	PubSub          *PubSubConfig        `json:"pub_sub"`
	IdempotencyKeys []string             `json:"idempotency_keys"`
	IdempotencyTTL  string               `json:"idempotency_ttl"`
}

func (us *UpdateSource) Validate() error {
	if err := util.Validate(us); err != nil {
		return err
	}

	if err := validateSourceVerifier(us.Verifier); err != nil {
		return err
	}

	if err := validateIdempotencyKeyFormat(us.IdempotencyKeys); err != nil {
		return err
	}

	return util.Validate(us)
}

type QueryListSource struct {
	// The source type e.g. http, pub_sub
	Type string `json:"type" example:"http"`
	// The custom source provider e.g. twitter, shopify
	Provider string `json:"provider" example:"twitter"`
	Pageable
}

type Pageable struct {
	Sort string `json:"sort"  example:"ASC | DESC"` // sort order
	// The number of items to return per page
	PerPage   int                     `json:"perPage" example:"20"`
	Direction datastore.PageDirection `json:"direction"`
	// A pagination cursor to fetch the previous page of a list
	PrevCursor string `json:"prev_page_cursor" example:"01H0JATTVCXZK8FRDX1M1JN3QY"`
	// A pagination cursor to fetch the next page of a list
	NextCursor string `json:"next_page_cursor" example:"01H0JA5MEES38RRK3HTEJC647K"`
}

type QueryListSourceResponse struct {
	datastore.Pageable
	*datastore.SourceFilter
}

func (ls *QueryListSource) Transform(r *http.Request) *QueryListSourceResponse {
	return &QueryListSourceResponse{
		Pageable: m.GetPageableFromContext(r.Context()),
		SourceFilter: &datastore.SourceFilter{
			Type:     r.URL.Query().Get("type"),
			Provider: r.URL.Query().Get("provider"),
		},
	}
}

type CustomResponse struct {
	Body        string `json:"body"`
	ContentType string `json:"content_type"`
}

type UpdateCustomResponse struct {
	Body        *string `json:"body"`
	ContentType *string `json:"content_type"`
}

type VerifierConfig struct {
	Type      datastore.VerifierType `json:"type,omitempty" valid:"supported_verifier~please provide a valid verifier type"`
	HMac      *HMac                  `json:"hmac"`
	BasicAuth *BasicAuth             `json:"basic_auth"`
	ApiKey    *ApiKey                `json:"api_key"`
}

func (vc *VerifierConfig) Transform() *datastore.VerifierConfig {
	if vc == nil {
		return nil
	}

	return &datastore.VerifierConfig{
		Type:      vc.Type,
		HMac:      vc.HMac.transform(),
		BasicAuth: vc.BasicAuth.transform(),
		ApiKey:    vc.ApiKey.transform(),
	}
}

type HMac struct {
	Header   string                 `json:"header" valid:"required"`
	Hash     string                 `json:"hash" valid:"supported_hash,required"`
	Secret   string                 `json:"secret" valid:"required"`
	Encoding datastore.EncodingType `json:"encoding" valid:"supported_encoding~please provide a valid encoding type,required"`
}

func (hm *HMac) transform() *datastore.HMac {
	if hm == nil {
		return nil
	}

	return &datastore.HMac{
		Header:   hm.Header,
		Hash:     hm.Hash,
		Secret:   hm.Secret,
		Encoding: hm.Encoding,
	}
}

type BasicAuth struct {
	UserName string `json:"username" valid:"required" `
	Password string `json:"password" valid:"required"`
}

func (ba *BasicAuth) transform() *datastore.BasicAuth {
	if ba == nil {
		return nil
	}

	return &datastore.BasicAuth{
		UserName: ba.UserName,
		Password: ba.Password,
	}
}

type ApiKey struct {
	HeaderValue string `json:"header_value" valid:"required"`
	HeaderName  string `json:"header_name" valid:"required"`
}

func (ak *ApiKey) transform() *datastore.ApiKey {
	if ak == nil {
		return nil
	}

	return &datastore.ApiKey{
		HeaderValue: ak.HeaderValue,
		HeaderName:  ak.HeaderName,
	}
}

type PubSubConfig struct {
	Type    datastore.PubSubType `json:"type"`
	Workers int                  `json:"workers"`
	Sqs     *SQSPubSubConfig     `json:"sqs"`
	Google  *GooglePubSubConfig  `json:"google"`
	Kafka   *KafkaPubSubConfig   `json:"kafka"`
}

func (pc *PubSubConfig) Transform() *datastore.PubSubConfig {
	if pc == nil {
		return nil
	}

	return &datastore.PubSubConfig{
		Type:    pc.Type,
		Workers: pc.Workers,
		Sqs:     pc.Sqs.transform(),
		Google:  pc.Google.transform(),
		Kafka:   pc.Kafka.transform(),
	}
}

type SQSPubSubConfig struct {
	AccessKeyID   string `json:"access_key_id"`
	SecretKey     string `json:"secret_key"`
	DefaultRegion string `json:"default_region"`
	QueueName     string `json:"queue_name"`
}

func (sc *SQSPubSubConfig) transform() *datastore.SQSPubSubConfig {
	if sc == nil {
		return nil
	}

	return &datastore.SQSPubSubConfig{
		AccessKeyID:   sc.AccessKeyID,
		SecretKey:     sc.SecretKey,
		DefaultRegion: sc.DefaultRegion,
		QueueName:     sc.QueueName,
	}
}

type GooglePubSubConfig struct {
	SubscriptionID string `json:"subscription_id"`
	ServiceAccount []byte `json:"service_account"`
	ProjectID      string `json:"project_id"`
}

func (gc *GooglePubSubConfig) transform() *datastore.GooglePubSubConfig {
	if gc == nil {
		return nil
	}

	return &datastore.GooglePubSubConfig{
		SubscriptionID: gc.SubscriptionID,
		ServiceAccount: gc.ServiceAccount,
		ProjectID:      gc.ProjectID,
	}
}

type KafkaPubSubConfig struct {
	Brokers         []string   `json:"brokers"`
	ConsumerGroupID string     `json:"consumer_group_id"`
	TopicName       string     `json:"topic_name"`
	Auth            *KafkaAuth `json:"auth"`
}

func (kc *KafkaPubSubConfig) transform() *datastore.KafkaPubSubConfig {
	if kc == nil {
		return nil
	}

	return &datastore.KafkaPubSubConfig{
		Brokers:         kc.Brokers,
		ConsumerGroupID: kc.ConsumerGroupID,
		TopicName:       kc.TopicName,
		Auth:            kc.Auth.transform(),
	}
}

type KafkaAuth struct {
	Type     string `json:"type"`
	Hash     string `json:"hash"`
	Username string `json:"username"`
	Password string `json:"password"`
	TLS      bool   `json:"tls"`
}

func (ka *KafkaAuth) transform() *datastore.KafkaAuth {
	if ka == nil {
		return nil
	}

	return &datastore.KafkaAuth{
		Type:     ka.Type,
		Username: ka.Username,
		Password: ka.Password,
		Hash:     ka.Hash,
		TLS:      ka.TLS,
	}
}

type SourceResponse struct {
	*datastore.Source
}
