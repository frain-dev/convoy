import {Injectable} from '@angular/core';
import {from, Observable} from 'rxjs';
import {map} from 'rxjs/operators';
import {HttpService} from 'src/app/services/http/http.service';
import axios from 'axios';
import {apiOrigin} from 'src/app/services/api-origin';
import {BillingStrategy} from 'src/app/models/billing.model';
import {BillingEndpoints} from './billing-endpoints';

export interface InvoiceRow {
  id: string;
  number: string;
  issuedOn: string;
  amount: string;
  status: string;
  dueDate: string;
  pdfLink?: string;
  hostedLink?: string;
  // Raw ISO invoice date, kept for deterministic sorting (the formatted
  // `issuedOn` is locale display only and not safe to parse back).
  issuedAtRaw?: string;
}

@Injectable({ providedIn: 'root' })
export class BillingInvoicesService {
  private billingStrategy: BillingStrategy = 'cloud';

  constructor(private httpService: HttpService) {}

  setBillingStrategy(strategy: BillingStrategy): void {
    this.billingStrategy = strategy;
  }

  getInvoices(): Observable<InvoiceRow[]> {
    return from(this.getInvoicesData()).pipe(
      map(invoices => this.formatInvoicesData(invoices))
    );
  }

  downloadInvoice(orgID: string, invoiceID: string): Observable<Blob> {
    return from(this.downloadInvoiceData(orgID, invoiceID));
  }

  private async getInvoicesData() {
    try {
      const orgId = this.getOrganisationId();
      const url = BillingEndpoints.billingUrl(this.billingStrategy, 'invoices', orgId);
      const response = await this.httpService.request({
        url,
        method: 'get',
        hideNotification: true
      });
      return response.data;
    } catch (error) {
      console.warn('Failed to load invoices data:', error);
      return [];
    }
  }

  private async downloadInvoiceData(orgID: string, invoiceID: string): Promise<Blob> {
    try {
      // Get auth token for the request
      const authToken = this.httpService.getAccessToken();

      // Both strategies download in-app: the backend proxies the provider PDF and
      // streams it back with auth, so the browser never hits the provider directly.
      const path = `${BillingEndpoints.billingUrl(this.billingStrategy, 'invoices', orgID)}/${invoiceID}/download`;

      // Build the URL
      const baseElement = document.querySelector('base');
      const baseHref = baseElement?.getAttribute('href') || '/';
      const rootPath = baseHref.replace(/\/$/, '');
      const apiURL = `${apiOrigin()}/ui`;
      const url = `${rootPath === '/' ? '' : rootPath}${apiURL}${path}`;

      // Make request with blob response type
      const response = await axios.get(url, {
        responseType: 'blob',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'X-Convoy-Version': '2024-04-01'
        }
      });

      return response.data;
    } catch (error: any) {
      console.error('Failed to download invoice:', error);
      if (error.response?.status === 404) {
        throw new Error('Invoice not found');
      } else if (error.response?.status === 403) {
        throw new Error('Unauthorized to download this invoice');
      } else {
        throw new Error('Failed to download invoice. Please try again.');
      }
    }
  }

  private formatInvoicesData(invoices: any[]): InvoiceRow[] {
    if (!invoices || invoices.length === 0) {
      return [
        { id: '', number: '', issuedOn: 'No invoices', amount: '$0', status: 'No data', dueDate: 'N/A' }
      ];
    }

    return invoices.map(invoice => {
      // Format amount from cents to dollars
      const amountInDollars = invoice.total_amount ? (invoice.total_amount / 100).toFixed(2) : '0.00';
      
      // Format invoice date
      let issuedOn = 'N/A';
      if (invoice.invoice_date) {
        try {
          issuedOn = new Date(invoice.invoice_date).toLocaleDateString('en-US', {
        month: '2-digit',
        day: '2-digit',
        year: 'numeric'
          });
        } catch (e) {
          console.warn('Invalid invoice_date:', invoice.invoice_date);
        }
      }
      
      // Use the real due date from the provider; fall back to invoice_date only when
      // an older record predates the stored due_date.
      let dueDate = 'N/A';
      const dateToUse = invoice.due_date || invoice.invoice_date;
      if (dateToUse) {
        try {
          dueDate = new Date(dateToUse).toLocaleDateString('en-US', {
        month: '2-digit',
        day: '2-digit',
        year: 'numeric'
          });
        } catch (e) {
          console.warn('Invalid date:', dateToUse);
        }
      }

      return {
        id: invoice.id || invoice.uid || '',
        number: invoice.number || '',
        issuedOn,
        amount: `$${amountInDollars}`,
        status: invoice.status ? (invoice.status.charAt(0).toUpperCase() + invoice.status.slice(1).toLowerCase()) : 'Unknown',
        dueDate,
        pdfLink: invoice.pdf_link,
        hostedLink: invoice.hosted_link,
        issuedAtRaw: invoice.invoice_date
      };
    });
  }

  private getOrganisationId(): string {
    const org = this.httpService.getOrganisation();
    return org ? org.uid : '';
  }
}
