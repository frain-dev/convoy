import { Component, OnInit } from '@angular/core';
import { BillingUsageService, UsageRow } from './billing-usage.service';

@Component({
  selector: 'app-billing-usage',
  templateUrl: './billing-usage.component.html',
  styleUrls: ['./billing-usage.component.scss']
})
export class BillingUsageComponent implements OnInit {
  isFetchingUsage = false;
  usageRows: UsageRow[] = [];
  tableHead = ['Name', 'Received', 'Sent', 'Total'];

  constructor(private usageService: BillingUsageService) {}

  ngOnInit() {
    this.fetchUsage();
  }

  fetchUsage() {
    this.isFetchingUsage = true;
    this.usageService.getUsage().subscribe(rows => {
      this.usageRows = rows;
      this.isFetchingUsage = false;
    });
  }
} 