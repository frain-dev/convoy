import { Injectable } from '@angular/core';
import { Observable, from } from 'rxjs';
import { map } from 'rxjs/operators';
import { HttpService } from 'src/app/services/http/http.service';

export interface UsageRow {
  name: string;
  sent: string;
  received: string;
  total: string;
}

@Injectable({ providedIn: 'root' })
export class BillingUsageService {
  constructor(private httpService: HttpService) {}

  getUsage(): Observable<UsageRow[]> {
    return from(this.getUsageData()).pipe(
      map(usage => this.formatUsageData(usage))
    );
  }

  private async getUsageData() {
    try {
      const orgId = this.getOrganisationId();
      const response = await this.httpService.request({
        url: `/organisations/${orgId}/billing/usage`,
        method: 'get',
        hideNotification: true
      });
      return response.data;
    } catch (error) {
      console.warn('Failed to load usage data:', error);
      return null;
    }
  }

  private formatUsageData(usage: any): UsageRow[] {
    if (!usage) {
      return [
        { name: 'Webhook event volume', sent: '0', received: '0', total: '0' },
        { name: 'Webhook event size', sent: '0 GB', received: '0 GB', total: '0 GB' }
      ];
    }

    return [
      { 
        name: 'Webhook event volume', 
        sent: usage.events?.toLocaleString() || '0', 
        received: usage.deliveries?.toLocaleString() || '0', 
        total: ((usage.events || 0) + (usage.deliveries || 0)).toLocaleString() 
      },
      { 
        name: 'Webhook event size', 
        sent: `${((usage.bandwidth || 0) / 1024 / 1024).toFixed(1)} GB`, 
        received: `${((usage.bandwidth || 0) / 1024 / 1024).toFixed(1)} GB`, 
        total: `${(((usage.bandwidth || 0) * 2) / 1024 / 1024).toFixed(1)} GB` 
      }
    ];
  }

  private getOrganisationId(): string {
    const org = localStorage.getItem('CONVOY_ORG');
    return org ? JSON.parse(org).uid : '';
  }
} 