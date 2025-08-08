import {Injectable} from '@angular/core';
import {from, Observable} from 'rxjs';
import {map} from 'rxjs/operators';
import {HttpService} from 'src/app/services/http/http.service';
import {environment} from 'src/environments/environment';

export interface InvoiceRow {
  id: string;
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

  downloadInvoice(invoiceId: string): Observable<Blob> {
    return from(this.downloadInvoiceData(invoiceId));
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

  private async downloadInvoiceData(invoiceId: string): Promise<Blob> {
    try {
      const orgId = this.getOrganisationId();
      const authToken = this.httpService.authDetails()?.access_token;

      if (!authToken) {
        throw new Error('No authentication token available');
      }

      const baseUrl = environment.production ? location.origin : 'http://localhost:5005';
      const url = `${baseUrl}/ui/organisations/${orgId}/billing/invoices/${invoiceId}/download`;

      const response = await fetch(url, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'X-Convoy-Version': '2024-04-01'
        }
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      return await response.blob();
    } catch (error) {
      console.error('Failed to download invoice:', error);
      throw error;
    }
  }

  private formatInvoicesData(invoices: any[]): InvoiceRow[] {
    if (!invoices || invoices.length === 0) {
      return [
        { id: '', issuedOn: 'No invoices', amount: '$0', status: 'No data', dueDate: 'N/A' }
      ];
    }

    return invoices.map(invoice => ({
      id: invoice.id || invoice.uid || '',
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
