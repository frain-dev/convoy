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

  async ngOnInit() {
    await this.overviewService.waitForBootstrap();
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
    // Get organisation ID from localStorage
    const org = localStorage.getItem('CONVOY_ORG');
    if (!org) {
      this.generalService.showNotification({
        message: 'Organisation not found. Please refresh the page.',
        style: 'error'
      });
      return;
    }

    let orgID: string;
    try {
      const orgData = JSON.parse(org);
      orgID = orgData.uid || '';
      if (!orgID) {
        throw new Error('Invalid organisation data');
      }
    } catch (error) {
      this.generalService.showNotification({
        message: 'Invalid organisation data. Please refresh the page.',
        style: 'error'
      });
      return;
    }

    // Find the invoice row to get the invoice number for filename
    const invoiceRow = this.invoiceRows.find(row => row.id === invoiceId);
    const invoiceNumber = invoiceRow?.number || invoiceId;

    this.invoicesService.downloadInvoice(orgID, invoiceId).subscribe({
      next: (blob) => {
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
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
        const errorMessage = error instanceof Error ? error.message : 'Failed to download invoice. Please try again.';
        this.generalService.showNotification({
          message: errorMessage,
          style: 'error'
        });
      }
    });
  }
}
