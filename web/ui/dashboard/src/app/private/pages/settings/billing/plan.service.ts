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

  // Fallback data structure for when no plans are configured
  getDefaultPlanComparison(): { plans: Plan[] } {
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
}
