import {Component, EventEmitter, Input, OnChanges, OnInit, Output, SimpleChanges} from '@angular/core';
import {BillingOverview, BillingOverviewService} from './billing-overview.service';
import {CardIconService} from './card-icon.service';

@Component({
  selector: 'app-billing-overview',
  templateUrl: './billing-overview.component.html',
  styleUrls: ['./billing-overview.component.scss']
})
export class BillingOverviewComponent implements OnInit, OnChanges {
  @Output() openPaymentDetails = new EventEmitter<void>();
  @Input() refreshTrigger: number = 0;

  overview: BillingOverview | null = null;
  isLoading = true;

  constructor(
    private overviewService: BillingOverviewService,
    private cardIconService: CardIconService
  ) {}

  ngOnInit() {
    this.loadOverview();
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['refreshTrigger'] && !changes['refreshTrigger'].firstChange) {
      this.refreshOverview();
    }
  }

  private loadOverview() {
    this.overviewService.getOverview().subscribe({
      next: (data) => {
        this.overview = data;
        this.isLoading = false;
      },
      error: (error) => {
        console.warn('Failed to load overview:', error);
        this.isLoading = false;
      }
    });
  }

  refreshOverview() {
    this.loadOverview();
  }

  getCardIconSvg() {
    return this.cardIconService.getCardIconSvg(this.overview?.payment?.brand);
  }

  onOpenPaymentDetails() {
    this.openPaymentDetails.emit();
  }

  onManagePlan(event: Event) {
    event.preventDefault();
    // No-op for now - plan management functionality not yet implemented
    console.log('Plan management not yet implemented');
  }
}
