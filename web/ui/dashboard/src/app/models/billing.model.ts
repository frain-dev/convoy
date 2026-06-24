export type BillingStrategy = 'oss' | 'cloud' | 'licensed_self_hosted';

export const CHECKOUT_STATUS = {
	COMPLETED: 'completed',
	PENDING: 'pending',
	EXPIRED: 'expired',
	ACTIVE: 'active'
} as const;

export interface ApiResponse<T> {
	data: T;
	message?: string;
	status?: boolean;
}

export interface SubscriptionPlan {
	id?: string;
	name?: string;
	price?: number;
	currency?: string;
}

export interface Subscription {
	id?: string;
	status?: string;
	plan?: SubscriptionPlan;
	// Billing cycle bounds (ISO 8601) reported by the billing service; used to
	// scope the usage query to the displayed period.
	current_period_start?: string;
	current_period_end?: string;
}

export interface SelfHostedActiveCheckout {
	attempt_id?: string;
	checkout_id?: string;
	checkout_url?: string;
}

export interface SelfHostedBillingConfig {
	license_configured?: boolean;
	// Server-resolved: a prior guest purchase exists, so checkout is a resubscribe.
	resubscribe?: boolean;
	active_checkout?: SelfHostedActiveCheckout | null;
}

export interface TaxIdType {
	type: string;
	example?: string;
}

export interface LicenseFeature {
	allowed: boolean;
}

export interface LicenseLimit {
	current: number;
	limit: number;
	available: boolean;
	limit_reached: boolean;
}

export type LicenseData = Record<string, LicenseFeature | LicenseLimit | boolean>;
