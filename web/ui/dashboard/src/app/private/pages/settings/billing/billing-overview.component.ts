import {Component, EventEmitter, Input, OnChanges, OnInit, Output, SimpleChanges} from '@angular/core';
import {BillingOverview} from './billing-overview.service';
import {CardIconService} from './card-icon.service';

@Component({
    selector: 'app-billing-overview',
    templateUrl: './billing-overview.component.html',
    styleUrls: ['./billing-overview.component.scss'],
    standalone: false
})
export class BillingOverviewComponent implements OnInit, OnChanges {
  @Output() openPaymentDetails = new EventEmitter<void>();
  @Output() openManagePlan = new EventEmitter<void>();
  @Output() startTrialConversion = new EventEmitter<void>();
  @Output() payPastDue = new EventEmitter<void>();
  @Input() refreshTrigger: number = 0;
  @Input() overview: BillingOverview | null = null;
  @Input() isLoading: boolean = false;
  @Input() isConverting: boolean = false;

  constructor(
    private cardIconService: CardIconService
  ) {}

  // True once billing data has loaded and the subscription is past due /
  // suspended. Gated on !isLoading so the banner never flashes over the
  // skeleton loaders before the overview resolves.
  get isPastDue(): boolean {
    return !this.isLoading && !!this.overview?.pastDue;
  }

  ngOnInit() {
  }

  ngOnChanges(_changes: SimpleChanges) {
  }

  getCardIconSvg() {
    return this.cardIconService.getCardIconSvg(this.overview?.payment?.brand);
  }

  onOpenPaymentDetails() {
    this.openPaymentDetails.emit();
  }

  onManagePlan(event: Event) {
    event.preventDefault();
    this.openManagePlan.emit();
  }

  onStartTrialConversion(event: Event) {
    event.preventDefault();
    if (this.isConverting) return;
    this.startTrialConversion.emit();
  }

  onPayPastDue(event: Event) {
    event.preventDefault();
    this.payPastDue.emit();
  }
}
