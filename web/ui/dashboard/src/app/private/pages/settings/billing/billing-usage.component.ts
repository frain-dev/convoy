import { Component, Input, OnInit } from '@angular/core';
import { UsageRow } from './billing-usage.service';

@Component({
  selector: 'app-billing-usage',
  templateUrl: './billing-usage.component.html',
  styleUrls: ['./billing-usage.component.scss']
})
export class BillingUsageComponent implements OnInit {
  @Input() usageRows: UsageRow[] = [];
  @Input() isFetchingUsage: boolean = false;
  tableHead = ['Name', 'Received', 'Sent', 'Total'];

  constructor() {}

  ngOnInit() {
  }
} 