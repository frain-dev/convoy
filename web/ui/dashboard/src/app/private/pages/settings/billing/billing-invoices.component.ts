import {Component, OnInit} from '@angular/core';
import {BillingInvoicesService, InvoiceRow} from './billing-invoices.service';
import {GeneralService} from 'src/app/services/general/general.service';

@Component({
  selector: 'app-billing-invoices',
  templateUrl: './billing-invoices.component.html',
  styleUrls: ['./billing-invoices.component.scss']
})
export class BillingInvoicesComponent implements OnInit {
  isFetchingInvoices = false;
  invoiceRows: InvoiceRow[] = [];
  tableHead = ['Issued on', 'Amount', 'Status', 'Due date', ''];

  constructor(
    private invoicesService: BillingInvoicesService,
    private generalService: GeneralService
  ) {}

  ngOnInit() {
    this.fetchInvoices();
  }

  fetchInvoices() {
    this.isFetchingInvoices = true;
    this.invoicesService.getInvoices().subscribe(rows => {
      this.invoiceRows = rows;
      this.isFetchingInvoices = false;
    });
  }

  downloadInvoice(invoiceId: string) {
    if (!invoiceId) {
      this.generalService.showNotification({
        message: 'Invoice ID not available',
        style: 'error'
      });
      return;
    }

    this.invoicesService.downloadInvoice(invoiceId).subscribe({
      next: (blob) => {
        // Create a download link
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `invoice-${invoiceId}.pdf`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(url);

        this.generalService.showNotification({
          message: 'Invoice downloaded successfully',
          style: 'success'
        });
      },
      error: (error) => {
        console.error('Download failed:', error);
        this.generalService.showNotification({
          message: 'Failed to download invoice. Please try again.',
          style: 'error'
        });
      }
    });
  }
}
