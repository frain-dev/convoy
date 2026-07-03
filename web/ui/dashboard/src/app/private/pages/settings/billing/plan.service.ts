import {Injectable} from '@angular/core';
import {from, Observable} from 'rxjs';
import {HttpService} from 'src/app/services/http/http.service';

export interface PlanFeature {
  name: string;
  description?: string;
  category: 'core' | 'security' | 'support';
  value: string;
}

export interface PlanPricingOption {
  interval: string;
  amount_cents?: number;
  currency?: string;
  trial_days?: number;
}

export interface Plan {
  id: string;
  key?: string;
  name: string;
  product_type?: string;
  description: string;
  price: number;
  currency: string;
  interval: string;
  intervals?: string[];
  pricing_options?: PlanPricingOption[];
  features: PlanFeature[];
  checkout_enabled?: boolean;
  requires_contact?: boolean;
  isPopular?: boolean;
  isCurrent?: boolean;
}

@Injectable({
  providedIn: 'root'
})
export class PlanService {
  constructor(private httpService: HttpService) {}

  getPlans(): Observable<{ data: Plan[] }> {
    return from(this.httpService.request({
      url: '/billing/plans',
      method: 'get'
    }));
  }

  // Starts a time-limited cloud trial for the org. The billing service defaults the
  // plan/interval/length and enforces the one-trial-per-org rule; an empty body is
  // intentional. Shared by the billing page and the projects empty state.
  startTrial(orgId: string): Promise<{ status: boolean; message: string; data: any }> {
    return this.httpService.request({
      url: `/billing/organisations/${orgId}/subscriptions/trial`,
      method: 'post',
      body: {}
    });
  }

  startSelfHostedTrial(email: string, host: string): Promise<{ status: boolean; message: string; data: any }> {
    return this.httpService.request({
      url: '/billing/sh_trial/start',
      method: 'post',
      body: { email, host }
    });
  }

  // Fallback data structure for when no plans are configured
  getDefaultPlanComparison(isSelfHosted = false): { plans: Plan[] } {
    if (isSelfHosted) {
      return this.getDefaultSelfHostedPlanComparison();
    }
    return {
      plans: [
        {
          id: 'pro',
          name: 'Pro',
          description: 'Perfect for growing businesses',
          price: 99,
          currency: 'USD',
          interval: 'month',
          features: [
            { name: 'Static IPs', category: 'core', value: 'Add-on ($100/month)' },
            { name: 'Incoming & Outgoing Webhooks', category: 'core', value: 'Supported' },
            { name: 'Rate Limit', category: 'core', value: 'Supported' },
            { name: 'Retries', category: 'core', value: 'Supported' },
            { name: 'Portal Links', category: 'core', value: 'Supported' },
            { name: 'Message Broker Support', category: 'core', value: 'Supported' },
            { name: 'Endpoint Circuit Breaking', category: 'core', value: 'Supported' },
            { name: 'Webhook Transformation with JS', category: 'core', value: 'Supported' },
            { name: 'Google SSO', category: 'security', value: 'Supported' },
            { name: 'SAML', category: 'security', value: 'Unsupported' },
            { name: 'Role based Access Control', category: 'security', value: 'Unsupported' },
            { name: 'SOC 2', category: 'security', value: 'Supported' },
            { name: 'VPC Peering & Private Networking', category: 'security', value: 'Unsupported' },
            { name: 'Email', category: 'support', value: 'Supported' },
            { name: 'Response SLA', category: 'support', value: 'Unsupported' },
            { name: 'Solutions Engineering', category: 'support', value: 'Unsupported' }
          ]
        },
        {
          id: 'enterprise',
          key: 'enterprise',
          name: 'Enterprise',
          description: 'For large organizations',
          price: 0,
          currency: 'USD',
          interval: 'month',
          checkout_enabled: false,
          requires_contact: true,
          features: [
            { name: 'Static IPs', category: 'core', value: 'Supported' },
            { name: 'Incoming & Outgoing Webhooks', category: 'core', value: 'Supported' },
            { name: 'Rate Limit', category: 'core', value: 'Supported' },
            { name: 'Retries', category: 'core', value: 'Supported' },
            { name: 'Portal Links', category: 'core', value: 'Supported' },
            { name: 'Message Broker Support', category: 'core', value: 'Supported' },
            { name: 'Endpoint Circuit Breaking', category: 'core', value: 'Supported' },
            { name: 'Webhook Transformation with JS', category: 'core', value: 'Supported' },
            { name: 'Google SSO', category: 'security', value: 'Supported' },
            { name: 'SAML', category: 'security', value: 'Supported' },
            { name: 'Role based Access Control', category: 'security', value: 'Supported' },
            { name: 'SOC 2', category: 'security', value: 'Supported' },
            { name: 'VPC Peering & Private Networking', category: 'security', value: 'Supported' },
            { name: 'Email', category: 'support', value: 'Supported' },
            { name: 'Response SLA', category: 'support', value: 'Supported' },
            { name: 'Solutions Engineering', category: 'support', value: 'Supported' }
          ]
        }
      ]
    };
  }

  /** Default feature rows for self-hosted Premium / Enterprise comparison tables. */
  getDefaultSelfHostedPlanComparison(): { plans: Plan[] } {
    const sharedCore: PlanFeature[] = [
      { name: 'Incoming & Outgoing Webhooks', category: 'core', value: 'Supported' },
      { name: 'Rate Limit', category: 'core', value: 'Supported' },
      { name: 'Retries', category: 'core', value: 'Supported' },
      { name: 'Portal Links', category: 'core', value: 'Supported' },
      { name: 'Message Broker Support', category: 'core', value: 'Supported' },
      { name: 'Endpoint Circuit Breaking', category: 'core', value: 'Supported' },
      { name: 'Webhook Transformation with JS', category: 'core', value: 'Supported' },
      { name: 'Asynq queue monitoring', category: 'core', value: 'Supported' },
      { name: 'Agent execution mode', category: 'core', value: 'Supported' },
      { name: 'Forward proxy', category: 'core', value: 'Supported' },
      { name: 'Prometheus metrics', category: 'core', value: 'Supported' },
      { name: 'Performance tuning', category: 'core', value: 'Supported' },
      { name: 'Advanced storage', category: 'core', value: 'Supported' },
      { name: 'Advanced webhook retention', category: 'core', value: 'Supported' },
      { name: 'Advanced webhook subscription', category: 'core', value: 'Supported' }
    ];
    const sharedSecurity: PlanFeature[] = [
      { name: 'Google SSO', category: 'security', value: 'Supported' },
      { name: 'SAML', category: 'security', value: 'Unsupported' },
      { name: 'Role based Access Control', category: 'security', value: 'Supported' },
      { name: 'IP rules', category: 'security', value: 'Supported' },
      { name: 'OAuth2 endpoint auth', category: 'security', value: 'Supported' },
      { name: 'SOC 2', category: 'security', value: 'Supported' }
    ];
    const sharedSupport: PlanFeature[] = [
      { name: 'Email', category: 'support', value: 'Supported' },
      { name: 'Response SLA', category: 'support', value: 'Unsupported' },
      { name: 'Solutions Engineering', category: 'support', value: 'Unsupported' }
    ];

    return {
      plans: [
        {
          id: 'self_hosted_premium',
          key: 'self_hosted_premium',
          name: 'Self-Hosted Premium',
          description: 'Premium self-hosted plan',
          price: 2499,
          currency: 'USD',
          interval: 'month',
          features: [...sharedCore, ...sharedSecurity, ...sharedSupport]
        },
        {
          id: 'self_hosted_enterprise',
          key: 'self_hosted_enterprise',
          name: 'Self-Hosted Enterprise',
          description: 'Enterprise self-hosted plan',
          price: 0,
          currency: 'USD',
          interval: 'month',
          checkout_enabled: false,
          requires_contact: true,
          features: [
            ...sharedCore,
            { name: 'SAML', category: 'security', value: 'Supported' },
            { name: 'Role based Access Control', category: 'security', value: 'Supported' },
            { name: 'IP rules', category: 'security', value: 'Supported' },
            { name: 'OAuth2 endpoint auth', category: 'security', value: 'Supported' },
            { name: 'Google SSO', category: 'security', value: 'Supported' },
            { name: 'SOC 2', category: 'security', value: 'Supported' },
            { name: 'Mutual TLS', category: 'security', value: 'Supported' },
            { name: 'Enterprise SSO', category: 'security', value: 'Supported' },
            { name: 'Email', category: 'support', value: 'Supported' },
            { name: 'Response SLA', category: 'support', value: 'Supported' },
            { name: 'Solutions Engineering', category: 'support', value: 'Supported' }
          ]
        }
      ]
    };
  }
}
