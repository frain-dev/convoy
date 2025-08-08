import { Injectable } from '@angular/core';
import { Observable, from } from 'rxjs';
import { map } from 'rxjs/operators';
import { HttpService } from 'src/app/services/http/http.service';

export interface InvoiceRow {
  issuedOn: string;
  amount: string;
  status: string;
  dueDate: string;
}

@Injectable({ providedIn: 'root' })
export class BillingInvoicesService {
  constructor(private httpService: HttpService) {}

  getInvoices(): Observable<InvoiceRow[]> {
    return from(this.getInvoicesData()).pipe(
      map(invoices => this.formatInvoicesData(invoices))
    );
  }

  private async getInvoicesData() {
    try {
      const orgId = this.getOrganisationId();
      const response = await this.httpService.request({
        url: `/organisations/${orgId}/billing/invoices`,
        method: 'get',
        hideNotification: true
      });
      return response.data;
    } catch (error) {
      console.warn('Failed to load invoices data:', error);
      return [];
    }
  }

  private formatInvoicesData(invoices: any[]): InvoiceRow[] {
    if (!invoices || invoices.length === 0) {
      return [
        { issuedOn: 'No invoices', amount: '$0', status: 'No data', dueDate: 'N/A' }
      ];
    }

    return invoices.map(invoice => ({
      issuedOn: new Date(invoice.created_at).toLocaleDateString('en-US', { 
        month: '2-digit', 
        day: '2-digit', 
        year: 'numeric' 
      }),
      amount: `$${invoice.amount}`,
      status: invoice.status.charAt(0).toUpperCase() + invoice.status.slice(1),
      dueDate: new Date(invoice.due_date).toLocaleDateString('en-US', { 
        month: '2-digit', 
        day: '2-digit', 
        year: 'numeric' 
      })
    }));
  }

  private getOrganisationId(): string {
    const org = localStorage.getItem('CONVOY_ORG');
    return org ? JSON.parse(org).uid : '';
  }
} 