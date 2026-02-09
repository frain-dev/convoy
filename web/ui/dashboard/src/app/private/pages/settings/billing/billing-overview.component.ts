import {Component, EventEmitter, Input, OnChanges, OnInit, Output, SimpleChanges} from '@angular/core';
import {BillingOverview} from './billing-overview.service';
import {CardIconService} from './card-icon.service';

@Component({
  selector: 'app-billing-overview',
  templateUrl: './billing-overview.component.html',
  styleUrls: ['./billing-overview.component.scss']
})
export class BillingOverviewComponent implements OnInit, OnChanges {
  @Output() openPaymentDetails = new EventEmitter<void>();
  @Output() openManagePlan = new EventEmitter<void>();
  @Input() refreshTrigger: number = 0;
  @Input() overview: BillingOverview | null = null;
  @Input() isLoading: boolean = false;

  constructor(
    private cardIconService: CardIconService
  ) {}

  ngOnInit() {
  }

  ngOnChanges(changes: SimpleChanges) {
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
}
