import {Component, OnInit} from '@angular/core';
import {BillingInvoicesService, InvoiceRow} from './billing-invoices.service';
import {GeneralService} from 'src/app/services/general/general.service';
import {BillingOverviewService} from './billing-overview.service';

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
    private generalService: GeneralService,
    private overviewService: BillingOverviewService
  ) {}

  ngOnInit() {
    this.overviewService.ensureBillingReady().then(() => {
      this.fetchInvoices();
    });
  }

  fetchInvoices() {
    this.isFetchingInvoices = true;
    this.invoicesService.getInvoices().subscribe(rows => {
      this.invoiceRows = rows;
      this.isFetchingInvoices = false;
    });
  }

  downloadInvoice(invoiceId: string) {
    // Find the invoice row to get the pdf_link
    const invoiceRow = this.invoiceRows.find(row => row.id === invoiceId);
    
    if (!invoiceRow || !invoiceRow.pdfLink) {
      this.generalService.showNotification({
        message: 'Invoice PDF link not available',
        style: 'error'
      });
      return;
    }

    this.invoicesService.downloadInvoice(invoiceRow.pdfLink).subscribe({
      next: (blob) => {
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        // Use invoice number for filename, fallback to ID if number not available
        const invoiceNumber = invoiceRow.number || invoiceId;
        // Convert to lowercase and replace spaces/hyphens if needed (e.g., "INV-1493" -> "inv-1493")
        const filename = invoiceNumber.toLowerCase().replace(/\s+/g, '-');
        link.download = `${filename}.pdf`;
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
