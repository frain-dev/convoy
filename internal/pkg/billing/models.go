package billing

type BillingOrganisation struct {
	ID             string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	ExternalID     string `json:"external_id,omitempty"`
	BillingEmail   string `json:"billing_email,omitempty"`
	Host           string `json:"host,omitempty"`
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

type UpdateOrganisationTaxIDRequest struct {
	TaxIDType string `json:"tax_id_type,omitempty"`
	TaxNumber string `json:"tax_number,omitempty"`
}

type UpdateOrganisationAddressRequest struct {
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
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Subscription struct {
	ID        string `json:"id,omitempty"`
	Status    string `json:"status,omitempty"`
	PlanID    string `json:"plan_id,omitempty"`
	Plan      *Plan  `json:"plan,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type Invoice struct {
	ID          string `json:"id,omitempty"`
	Number      string `json:"number,omitempty"`
	InvoiceDate string `json:"invoice_date,omitempty"`
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
}
