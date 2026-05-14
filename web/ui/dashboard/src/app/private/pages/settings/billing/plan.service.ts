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
  name: string;
  product_type?: string;
  description: string;
  price: number;
  currency: string;
  interval: string;
  intervals?: string[];
  pricing_options?: PlanPricingOption[];
  features: PlanFeature[];
  isPopular?: boolean;
  isCurrent?: boolean;
}

@Injectable({
  providedIn: 'root'
})
export class PlanService {
  constructor(private httpService: HttpService) {}

  getPlans(orgId?: string): Observable<{ data: Plan[] }> {
    return from(this.httpService.request({
      url: orgId ? `/billing/plans?org_id=${encodeURIComponent(orgId)}` : '/billing/plans',
      method: 'get'
    }));
  }
}
