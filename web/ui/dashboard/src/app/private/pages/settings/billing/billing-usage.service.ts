import {Injectable} from '@angular/core';
import {from, Observable} from 'rxjs';
import {map} from 'rxjs/operators';
import {HttpService} from 'src/app/services/http/http.service';

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
    return from(this.getUsageData()).pipe(map(usage => this.formatUsageData(usage)));
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

  private formatBytes(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let idx = 0;
    let value = bytes;

    while (value >= 1024 && idx < units.length - 1) {
      value = value / 1024;
      idx++;
    }

    const fixed = value >= 100 ? value.toFixed(0) : value.toFixed(1);
    return `${fixed} ${units[idx]}`;
  }

  private formatUsageData(usage: any): UsageRow[] {
    const sentVolume = Number(usage?.sent?.volume || 0);       // outgoing deliveries count
    const receivedVolume = Number(usage?.received?.volume || 0); // incoming events count
    const sentBytes = Number(usage?.sent?.bytes || 0);         // outgoing bytes (egress)
    const receivedBytes = Number(usage?.received?.bytes || 0); // incoming bytes (ingress)

    return [
      {
        name: 'Webhook event volume',
        sent: sentVolume.toLocaleString(),
        received: receivedVolume.toLocaleString(),
        total: (sentVolume + receivedVolume).toLocaleString()
      },
      {
        name: 'Webhook event size',
        sent: this.formatBytes(sentBytes),
        received: this.formatBytes(receivedBytes),
        total: this.formatBytes(sentBytes + receivedBytes)
      }
    ];
  }

  private getOrganisationId(): string {
    const org = localStorage.getItem('CONVOY_ORG');
    return org ? JSON.parse(org).uid : '';
  }
}
