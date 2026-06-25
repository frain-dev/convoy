package billing

// WorkspaceConfigData is the workspace_config API response.
type WorkspaceConfigData struct {
	ExternalID   string `json:"external_id"`
	LicenseKey   string `json:"license_key"`
	SSOAvailable bool   `json:"sso_available"`
}

type BillingOrganisation struct {
	ID             string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	Slug           string `json:"slug,omitempty"`
	ExternalID     string `json:"external_id,omitempty"`
	BillingEmail   string `json:"billing_email,omitempty"`
	BillingName    string `json:"billing_name,omitempty"`
	Host           string `json:"host,omitempty"`
	LicenseKey     string `json:"license_key,omitempty"`
	TaxIDType      string `json:"tax_id_type,omitempty"`
	TaxNumber      string `json:"tax_number,omitempty"`
	BillingAddress string `json:"billing_address,omitempty"`
	BillingCity    string `json:"billing_city,omitempty"`
	BillingState   string `json:"billing_state,omitempty"`
	BillingZip     string `json:"billing_zip,omitempty"`
	BillingCountry string `json:"billing_country,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	DeletedAt      string `json:"deleted_at,omitempty"`
}

type OrganisationLicense struct {
	ID              string               `json:"id,omitempty"`
	Key             string               `json:"key,omitempty"`
	KeygenLicenseID string               `json:"keygen_license_id,omitempty"`
	DeploymentType  string               `json:"deployment_type,omitempty"`
	InstanceURL     string               `json:"instance_url,omitempty"`
	Status          string               `json:"status,omitempty"`
	ExpiresAt       string               `json:"expires_at,omitempty"`
	CreatedAt       string               `json:"created_at,omitempty"`
	UpdatedAt       string               `json:"updated_at,omitempty"`
	Organisation    *BillingOrganisation `json:"organisation,omitempty"`
}

type UpdateOrganisationTaxIDRequest struct {
	TaxIDType string `json:"tax_id_type,omitempty"`
	TaxNumber string `json:"tax_number,omitempty"`
}

type UpdateOrganisationAddressRequest struct {
	BillingName    string `json:"billing_name,omitempty"`
	BillingAddress string `json:"billing_address,omitempty"`
	BillingCity    string `json:"billing_city,omitempty"`
	BillingState   string `json:"billing_state,omitempty"`
	BillingZip     string `json:"billing_zip,omitempty"`
	BillingCountry string `json:"billing_country,omitempty"`
}

type OnboardSubscriptionRequest struct {
	PlanID string `json:"plan_id,omitempty"`
	Host   string `json:"host,omitempty"`
}

type UpgradeSubscriptionRequest struct {
	PlanID string `json:"plan_id,omitempty"`
	Host   string `json:"host,omitempty"`
}

type Plan struct {
	ID              string          `json:"id,omitempty"`
	Key             string          `json:"key,omitempty"`
	Name            string          `json:"name,omitempty"`
	ProductType     string          `json:"product_type,omitempty"`
	Interval        string          `json:"interval,omitempty"`
	Intervals       []string        `json:"intervals,omitempty"`
	PricingOptions  []PricingOption `json:"pricing_options,omitempty"`
	CheckoutEnabled *bool           `json:"checkout_enabled,omitempty"`
	RequiresContact *bool           `json:"requires_contact,omitempty"`
}

type PricingOption struct {
	Interval    string `json:"interval,omitempty"`
	AmountCents int    `json:"amount_cents,omitempty"`
	Currency    string `json:"currency,omitempty"`
	TrialDays   int    `json:"trial_days,omitempty"`
}

type BillingSubscription struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
	PlanID string `json:"plan_id,omitempty"`
	Plan   *Plan  `json:"plan,omitempty"`
	// Billing cycle as reported by the billing service. ISO8601 strings, empty when there
	// is no upcoming invoice. Passed through so the dashboard shows the real subscription
	// period instead of deriving it from the usage month.
	CurrentPeriodStart string `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   string `json:"current_period_end,omitempty"`
	NextInvoiceDate    string `json:"next_invoice_date,omitempty"`
	CreatedAt          string `json:"created_at,omitempty"`
	UpdatedAt          string `json:"updated_at,omitempty"`
}

type Invoice struct {
	ID          string `json:"id,omitempty"`
	Number      string `json:"number,omitempty"`
	InvoiceDate string `json:"invoice_date,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
	Currency    string `json:"currency,omitempty"`
	Status      string `json:"status,omitempty"`
	HostedLink  string `json:"hosted_link,omitempty"`
	PDFLink     string `json:"pdf_link,omitempty"`
	PaidDate    string `json:"paid_date,omitempty"`
	TotalAmount int    `json:"total_amount,omitempty"`
}

type PaymentMethod struct {
	ID          string `json:"id,omitempty"`
	CardType    string `json:"card_type,omitempty"`
	ExpMonth    int    `json:"exp_month,omitempty"`
	ExpYear     int    `json:"exp_year,omitempty"`
	Last4       string `json:"last4,omitempty"`
	DefaultedAt string `json:"defaulted_at,omitempty"`
}

type SetupIntent struct {
	IntentSecret string `json:"intent_secret,omitempty"`
}

type Checkout struct {
	CheckoutURL string `json:"checkout_url,omitempty"`
	CheckoutID  string `json:"checkout_id,omitempty"`
	AttemptID   string `json:"attempt_id,omitempty"`
}

type StartGuestCheckoutRequest struct {
	Email             string `json:"email,omitempty"`
	PlanID            string `json:"plan_id,omitempty"`
	Interval          string `json:"interval,omitempty"`
	Host              string `json:"host,omitempty"`
	OrganisationName  string `json:"organisation_name,omitempty"`
	AttemptID         string `json:"attempt_id,omitempty"`
	CheckoutNonceHash string `json:"checkout_nonce_hash,omitempty"`
	// LicenseKey, when set, resubscribes the org for that key (empty = first purchase).
	LicenseKey string `json:"license_key,omitempty"`
}

type CompleteGuestCheckoutRequest struct {
	Token         string `json:"token,omitempty"`
	AttemptID     string `json:"attempt_id,omitempty"`
	CheckoutID    string `json:"checkout_id,omitempty"`
	CheckoutNonce string `json:"checkout_nonce,omitempty"`
}

type GuestCheckoutCompletion struct {
	Status     string `json:"status,omitempty"`
	LicenseKey string `json:"license_key,omitempty"`
	CheckoutID string `json:"checkout_id,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
}

type TaxIDType struct {
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
}

type CreateSetupIntentRequest struct {
	IntentSecret string `json:"intent_secret,omitempty"`
}

type UsageMetrics struct {
	Volume int64 `json:"volume,omitempty"`
	Bytes  int64 `json:"bytes,omitempty"`
}

type Usage struct {
	OrganisationID string       `json:"organisation_id,omitempty"`
	Period         string       `json:"period,omitempty"`
	Received       UsageMetrics `json:"received,omitempty"`
	Sent           UsageMetrics `json:"sent,omitempty"`
	CreatedAt      string       `json:"created_at,omitempty"`
	// Pending is true while usage is still being computed in the background. When
	// true, the metric values are not yet known and clients should render a
	// placeholder (e.g. "-") instead of treating zeros as real usage.
	Pending bool `json:"pending,omitempty"`
}
