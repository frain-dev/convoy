import {Injectable} from '@angular/core';
import {from, Observable} from 'rxjs';
import {map} from 'rxjs/operators';
import {HttpService} from 'src/app/services/http/http.service';
import axios from 'axios';
import {environment} from 'src/environments/environment';

export interface InvoiceRow {
  id: string;
  number: string;
  issuedOn: string;
  amount: string;
  status: string;
  dueDate: string;
  pdfLink?: string;
}

@Injectable({ providedIn: 'root' })
export class BillingInvoicesService {
  constructor(private httpService: HttpService) {}

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
      const response = await this.httpService.request({
        url: `/billing/organisations/${orgId}/invoices`,
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
      const authDetails = localStorage.getItem('CONVOY_AUTH_TOKENS');
      let authToken = '';
      if (authDetails && authDetails !== 'undefined') {
        const token = JSON.parse(authDetails);
        authToken = token.access_token || '';
      }

      // Build the URL
      const baseElement = document.querySelector('base');
      const baseHref = baseElement?.getAttribute('href') || '/';
      const rootPath = baseHref.replace(/\/$/, '');
      const apiURL = `${environment.production ? location.origin : 'http://localhost:5005'}/ui`;
      const url = `${rootPath === '/' ? '' : rootPath}${apiURL}/billing/organisations/${orgID}/invoices/${invoiceID}/download`;

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
      
      // Format due date (use paid_date if available, otherwise invoice_date)
      let dueDate = 'N/A';
      const dateToUse = invoice.paid_date || invoice.invoice_date;
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
        pdfLink: invoice.pdf_link
      };
    });
  }

  private getOrganisationId(): string {
    const org = localStorage.getItem('CONVOY_ORG');
    return org ? JSON.parse(org).uid : '';
  }
}
