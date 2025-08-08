import { Injectable } from '@angular/core';
import { Observable, of } from 'rxjs';

export interface InvoiceRow {
  issuedOn: string;
  amount: string;
  status: string;
  dueDate: string;
}

@Injectable({ providedIn: 'root' })
export class BillingInvoicesService {
  getInvoices(): Observable<InvoiceRow[]> {
    // Mocked data
    return of([
      { issuedOn: '08/10/2025', amount: '$99', status: 'Paid', dueDate: '10/11/2025' },
      { issuedOn: '08/10/2025', amount: '$99', status: 'Paid', dueDate: '10/11/2025' }
    ]);
  }
} 