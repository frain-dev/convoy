import {Injectable} from '@angular/core';
import {HttpService} from 'src/app/services/http/http.service';
import {BillingStrategy} from 'src/app/models/billing.model';
import {BillingEndpoints} from './billing-endpoints';

const MS_PER_MINUTE = 60 * 1000;
const MS_PER_HOUR = 60 * MS_PER_MINUTE;
const MS_PER_DAY = 24 * MS_PER_HOUR;
const MS_PER_YEAR = 365 * MS_PER_DAY;

/** Human-readable countdown until usage/billing reset with correct singular/plural units. */
export function formatTimeUntilReset(ms: number): string {
  if (ms <= 0) {
    return '1 minute';
  }
  if (ms < MS_PER_MINUTE) {
    return '1 minute';
  }
  if (ms < MS_PER_HOUR) {
    const minutes = Math.ceil(ms / MS_PER_MINUTE);
    return minutes === 1 ? '1 minute' : `${minutes} minutes`;
  }
  if (ms < MS_PER_DAY) {
    const hours = Math.ceil(ms / MS_PER_HOUR);
    return hours === 1 ? '1 hour' : `${hours} hours`;
  }
  if (ms < MS_PER_YEAR) {
    const days = Math.ceil(ms / MS_PER_DAY);
    return days === 1 ? '1 day' : `${days} days`;
  }
  const years = Math.ceil(ms / MS_PER_YEAR);
  return years === 1 ? '1 year' : `${years} years`;
}

export interface BillingOverview {
  plan: {
    name: string;
    price: string;
    trial?: boolean;
  };
  usage: {
    period: string;
    resetIn?: string;
  };
  payment: {
    last4: string;
    brand: string;
  } | null;
  // True when subscription is not in good standing (past due / suspended).
  pastDue?: boolean;
}

@Injectable({ providedIn: 'root' })
export class BillingOverviewService {
	bootstrapPromise: Promise<void> | null = null;
	private billingStrategy: BillingStrategy = 'cloud';

	constructor(private httpService: HttpService) {}

	setBootstrapPromise(promise: Promise<void> | null): void {
		this.bootstrapPromise = promise;
	}

	setBillingStrategy(strategy: BillingStrategy): void {
		this.billingStrategy = strategy;
	}

	// Canonical overview fetch (subscription first, then usage + payment).
	// Fail-closed: a subscription error returns null and skips the other calls.
	async getOverviewData() {
		try {
			const orgId = this.getOrganisationId();
			const subscriptionUrl = BillingEndpoints.billingUrl(this.billingStrategy, 'subscription', orgId);
			const paymentMethodsUrl = BillingEndpoints.billingUrl(this.billingStrategy, 'payment_methods', orgId);

			// First call - if this fails, don't make other calls
			const subscriptionResponse = await this.httpService.request({ url: subscriptionUrl, method: 'get', hideNotification: true });

			// Only make other calls if subscription call succeeded
			const [usageResponse, paymentResponse] = await Promise.all([
				this.billingStrategy === 'licensed_self_hosted'
					? Promise.resolve({ data: null })
					: this.httpService.request({ url: BillingEndpoints.billingUrl(this.billingStrategy, 'usage', orgId), method: 'get', hideNotification: true }),
				this.httpService.request({ url: paymentMethodsUrl, method: 'get', hideNotification: true })
			]);

			return {
				subscription: subscriptionResponse.data,
				usage: usageResponse.data,
				payment: paymentResponse.data
			};
		} catch (error) {
			console.warn('Failed to load overview data:', error);
			// Return null to indicate failure - don't make additional calls
			return null;
		}
	}

	formatOverviewData(data: any): BillingOverview {
    if (!data) {
      return {
        plan: { name: 'No plan', price: '$0' },
        usage: { period: 'No data' },
        payment: null,
        pastDue: false
      };
    }

    const subscription = data.subscription;
    const currentPlan = subscription?.plan || { name: 'No plan', price: 0, currency: 'USD' };
    // Match the billing page's own notion of an active subscription (an id, or a plan with
    // an id/name) rather than only the nested plan object. A subscription can carry cycle
    // fields at the root with the plan omitted; treating that as "no cycle" would wrongly
    // render "No active cycle" and skip the usage-month fallback below.
    const hasActiveSubscription = !!(subscription && (subscription.id || subscription.plan?.id || subscription.plan?.name));
    const isTrial = subscription?.trial === true;

    const usage = data.usage || { period: this.getCurrentPeriod() };
    // Resolve the period label and the reset countdown from a single source so the two
    // cards never disagree. Prefer the real billing cycle reported by the billing service
    // (current_period_start/end + next_invoice_date); only when both the cycle range and a
    // future next-invoice date are present do we use them together. Otherwise fall back to
    // the usage-month derivation for both, matching the backend's fail-open behaviour, so we
    // never render a wrong fixed "month 01 - next month 01" cycle or a period that resets on
    // an unrelated date. With no subscription there is no cycle, so we show neither.
    const billingPeriod = this.formatBillingCycle(data.subscription?.current_period_start, data.subscription?.current_period_end);
    const msFromCycle = this.millisecondsUntilDate(data.subscription?.next_invoice_date);

    let usagePeriod: string;
    let resetIn: string | undefined;
    if (!hasActiveSubscription) {
      usagePeriod = 'No active cycle';
      resetIn = undefined;
    } else if (isTrial) {
      const trialUsage = this.formatTrialUsagePeriod(data.subscription);
      if (trialUsage) {
        usagePeriod = trialUsage.period;
        resetIn = trialUsage.resetIn;
      } else if (billingPeriod && msFromCycle !== null) {
        usagePeriod = billingPeriod;
        resetIn = formatTimeUntilReset(msFromCycle);
      } else {
        usagePeriod = this.formatUsagePeriod(usage.period);
        const msFromUsage = this.millisecondsUntilResetFromPeriod(usage.period);
        resetIn = msFromUsage !== null ? formatTimeUntilReset(msFromUsage) : undefined;
      }
    } else if (billingPeriod && msFromCycle !== null) {
      usagePeriod = billingPeriod;
      resetIn = formatTimeUntilReset(msFromCycle);
    } else {
      usagePeriod = this.formatUsagePeriod(usage.period);
      const msFromUsage = this.millisecondsUntilResetFromPeriod(usage.period);
      resetIn = msFromUsage !== null ? formatTimeUntilReset(msFromUsage) : undefined;
    }
    // Find the default payment method, or fall back to the first one if no default is set
    const payment = data.payment && data.payment.length > 0 
      ? (data.payment.find((pm: any) => pm.defaulted_at !== null && pm.defaulted_at !== undefined) || data.payment[0])
      : null;

    const pastDue = hasActiveSubscription && this.isPastDueStatus(subscription?.status);

    return {
      plan: {
        name: currentPlan.name,
        price: `$${currentPlan.price}`,
        trial: isTrial
      },
      usage: {
        period: usagePeriod,
        resetIn
      },
      payment: payment && payment.last4 ? {
        last4: payment.last4,
        brand: payment.card_type || payment.brand || 'unknown'
      } : null,
      pastDue
    };
  }

  // A subscription is "past due" when its status is one of the explicit
  // not-good-standing values the billing service reports. Fail-soft: an unknown
  // or missing status is treated as good standing (no banner) so a healthy paid
  // org is never blocked behind a scary past-due card on a status we don't map.
  private isPastDueStatus(status?: string): boolean {
    if (!status) return false;
    const normalized = status.trim().toLowerCase().replace(/[\s-]+/g, '_');
    return (
      normalized === 'paused' ||
      normalized === 'past_due' ||
      normalized === 'unpaid' ||
      normalized === 'suspended'
    );
  }

  // Formats the provider billing cycle as "May 28 - Jun 28". Returns null when either
  // bound is missing or unparseable so the caller falls back to the usage-month period.
  // Includes the year on both bounds when they span different years (e.g. yearly terms),
  // otherwise "Jun 28 - Jun 28" would render as an identical, confusing range.
  private formatBillingCycle(startIso?: string, endIso?: string): string | null {
    if (!startIso || !endIso) return null;

    const start = new Date(startIso);
    const end = new Date(endIso);
    if (isNaN(start.getTime()) || isNaN(end.getTime())) return null;
    // Require start before end so the overview falls back to the usage-month label
    // on a degenerate cycle, matching usageRange() in billing-page.component.ts,
    // which sends no window (and thus aggregates the month) for inverted/equal bounds.
    if (start.getTime() >= end.getTime()) return null;

    const withYear = start.getUTCFullYear() !== end.getUTCFullYear();
    return `${this.formatCycleDay(start, withYear)} - ${this.formatCycleDay(end, withYear)}`;
  }

  // During trial, billing-cycle dates can reflect the paid SKU term (e.g. annual
  // self-hosted) rather than the trial window. Prefer trial_conversion_date when present.
  private formatTrialUsagePeriod(subscription: any): { period: string; resetIn?: string } | null {
    const conversionIso = subscription?.trial_conversion_date;
    if (!conversionIso) {
      return null;
    }

    const end = new Date(conversionIso);
    if (isNaN(end.getTime())) {
      return null;
    }

    let start = subscription?.current_period_start ? new Date(subscription.current_period_start) : new Date();
    if (isNaN(start.getTime()) || start.getTime() >= end.getTime()) {
      start = new Date(end);
    }

    // Annual (or other long) billing cycles must not define the trial window.
    const maxTrialMs = 31 * 24 * 60 * 60 * 1000;
    if (end.getTime() - start.getTime() > maxTrialMs) {
      start = new Date(end.getTime() - 14 * 24 * 60 * 60 * 1000);
    }

    const period = this.formatBillingCycle(start.toISOString(), end.toISOString())
      ?? `Through ${this.formatCycleDay(end, true)}`;
    const msUntilConversion = this.millisecondsUntilDate(conversionIso);
    const resetIn = msUntilConversion !== null ? formatTimeUntilReset(msUntilConversion) : undefined;
    return { period, resetIn };
  }

  // Billing-cycle boundaries from the billing service are UTC instants (often midnight UTC
  // on yearly terms). Format them in UTC so a user west of UTC doesn't see the previous
  // calendar day and disagree with the provider invoice date.
  private formatCycleDay(date: Date, withYear = false): string {
    const month = date.toLocaleDateString('en-US', { month: 'short', timeZone: 'UTC' });
    const day = date.getUTCDate().toString().padStart(2, '0');
    return withYear ? `${month} ${day}, ${date.getUTCFullYear()}` : `${month} ${day}`;
  }

  // Milliseconds from now until the next invoice date. Returns null when the date is missing
  // or unparseable so the caller falls back to the usage-month reset calculation.
  private millisecondsUntilDate(iso?: string): number | null {
    if (!iso) return null;

    const target = new Date(iso);
    if (isNaN(target.getTime())) return null;

    const diffMs = target.getTime() - Date.now();
    // A past invoice date means our cached cycle is stale; return null so the caller falls
    // back to the usage-month reset instead of showing a misleading countdown.
    if (diffMs < 0) return null;
    return diffMs;
  }

  private formatUsagePeriod(period: string): string {
    const [year, month] = period.split('-');
    const currentDate = new Date(parseInt(year), parseInt(month) - 1, 1);
    const nextDate = new Date(parseInt(year), parseInt(month), 1);

    const currentMonth = currentDate.toLocaleDateString('en-US', { month: 'short' });
    const currentDay = currentDate.getDate();
    const nextMonth = nextDate.toLocaleDateString('en-US', { month: 'short' });
    const nextDay = nextDate.getDate();

    return `${currentMonth} ${currentDay.toString().padStart(2, '0')} - ${nextMonth} ${nextDay.toString().padStart(2, '0')}`;
  }

  private getCurrentPeriod(): string {
    const now = new Date();
    const year = now.getFullYear();
    const month = (now.getMonth() + 1).toString().padStart(2, '0');
    return `${year}-${month}`;
  }

  private millisecondsUntilResetFromPeriod(period: string): number | null {
    const [year, month] = period.split('-');
    const currentDate = new Date();
    const currentPeriodStart = new Date(parseInt(year), parseInt(month) - 1, 1);
    const nextPeriodStart = new Date(parseInt(year), parseInt(month), 1);

    // If we're in the current period, calculate ms until next period.
    if (currentDate >= currentPeriodStart && currentDate < nextPeriodStart) {
      const diffMs = nextPeriodStart.getTime() - currentDate.getTime();
      return Math.max(0, diffMs);
    }

    // If we're past this period, return 0.
    return 0;
  }

  private getOrganisationId(): string {
    const org = this.httpService.getOrganisation();
    return org ? org.uid : '';
  }
}
