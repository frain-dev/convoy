import { Injectable } from '@angular/core';
import { Observable, of } from 'rxjs';

export interface UsageRow {
  name: string;
  sent: string;
  received: string;
  total: string;
}

@Injectable({ providedIn: 'root' })
export class BillingUsageService {
  getUsage(): Observable<UsageRow[]> {
    // Mocked data
    return of([
      { name: 'Webhook event volume', sent: '12,000', received: '5,000', total: '17,000' },
      { name: 'Webhook event size', sent: '0.5 GB', received: '12.2 GB', total: '12.7 GB' }
    ]);
  }
} 