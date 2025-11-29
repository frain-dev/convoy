import {Injectable} from '@angular/core';
import {from, Observable} from 'rxjs';
import {map} from 'rxjs/operators';
import {HttpService} from 'src/app/services/http/http.service';

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

  downloadInvoice(pdfLink: string): Observable<Blob> {
    return from(this.downloadInvoiceData(pdfLink));
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

  private async downloadInvoiceData(pdfLink: string): Promise<Blob> {
    try {
      const response = await fetch(pdfLink);
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
