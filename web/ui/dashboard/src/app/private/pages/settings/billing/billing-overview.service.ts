import {Injectable} from '@angular/core';
import {from, Observable} from 'rxjs';
import {map} from 'rxjs/operators';
import {HttpService} from 'src/app/services/http/http.service';

export interface BillingOverview {
  plan: {
    name: string;
    price: string;
  };
  usage: {
    period: string;
    daysUntilReset: number;
  };
  payment: {
    last4: string;
    brand: string;
  } | null;
}

@Injectable({ providedIn: 'root' })
export class BillingOverviewService {
  constructor(private httpService: HttpService) {}

  getOverview(): Observable<BillingOverview> {
    return from(this.getOverviewData()).pipe(
      map(data => this.formatOverviewData(data))
    );
  }

  async ensureBillingReady(): Promise<void> {
    const orgId = this.getOrganisationId();
    await this.httpService.request({ url: `/billing/organisations/${orgId}/subscription`, method: 'get', hideNotification: true });
  }

  private async getOverviewData() {
    try {
      const orgId = this.getOrganisationId();

      // First call - if this fails, don't make other calls
      const subscriptionResponse = await this.httpService.request({ url: `/billing/organisations/${orgId}/subscription`, method: 'get', hideNotification: true });

      // Only make other calls if subscription call succeeded
      const [usageResponse, paymentResponse] = await Promise.all([
        this.httpService.request({ url: `/billing/organisations/${orgId}/usage`, method: 'get', hideNotification: true }),
        this.httpService.request({ url: `/billing/organisations/${orgId}/payment_methods`, method: 'get', hideNotification: true })
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

  private formatOverviewData(data: any): BillingOverview {
    if (!data) {
      return {
        plan: { name: 'No plan', price: '$0' },
        usage: { period: 'No data', daysUntilReset: 0 },
        payment: null
      };
    }

    const currentPlan = data.subscription?.plan || { name: 'No plan', price: 0, currency: 'USD' };

    const usage = data.usage || { period: '2024-01' };
    const usagePeriod = this.formatUsagePeriod(usage.period);
    const daysUntilReset = this.calculateDaysUntilReset(usage.period);
    const payment = data.payment && data.payment.length > 0 ? data.payment[0] : null;

    return {
      plan: {
        name: currentPlan.name,
        price: `$${currentPlan.price}`
      },
      usage: {
        period: usagePeriod,
        daysUntilReset
      },
      payment: payment && payment.last4 ? {
        last4: payment.last4,
        brand: payment.card_type || payment.brand || 'unknown'
      } : null
    };
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

  private calculateDaysUntilReset(period: string): number {
    const [year, month] = period.split('-');
    const currentDate = new Date();
    const currentPeriodStart = new Date(parseInt(year), parseInt(month) - 1, 1);
    const nextPeriodStart = new Date(parseInt(year), parseInt(month), 1);

    // If we're in the current period, calculate days until next period
    if (currentDate >= currentPeriodStart && currentDate < nextPeriodStart) {
      const diffTime = nextPeriodStart.getTime() - currentDate.getTime();
      const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));
      return Math.max(0, diffDays);
    }

    // If we're past this period, return 0
    return 0;
  }

  private getOrganisationId(): string {
    const org = localStorage.getItem('CONVOY_ORG');
    return org ? JSON.parse(org).uid : '';
  }
}
