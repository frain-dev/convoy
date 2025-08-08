import { Component, OnInit } from '@angular/core';
import { BillingInvoicesService, InvoiceRow } from './billing-invoices.service';

@Component({
  selector: 'app-billing-invoices',
  templateUrl: './billing-invoices.component.html',
  styleUrls: ['./billing-invoices.component.scss']
})
export class BillingInvoicesComponent implements OnInit {
  isFetchingInvoices = false;
  invoiceRows: InvoiceRow[] = [];
  tableHead = ['Issued on', 'Amount', 'Status', 'Due date', ''];

  constructor(private invoicesService: BillingInvoicesService) {}

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
} 