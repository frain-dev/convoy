import {CUSTOM_ELEMENTS_SCHEMA} from '@angular/core';
import {ComponentFixture, TestBed} from '@angular/core/testing';
import {BillingOverviewComponent} from './billing-overview.component';
import {BillingOverview} from './billing-overview.service';
import {CardIconService} from './card-icon.service';

describe('BillingOverviewComponent past-due banner', () => {
	let fixture: ComponentFixture<BillingOverviewComponent>;
	let component: BillingOverviewComponent;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [BillingOverviewComponent],
			// The template uses <convoy-skeleton-loader>; we only assert on the plan
			// card content here, so ignore unknown elements instead of importing the UI module.
			schemas: [CUSTOM_ELEMENTS_SCHEMA],
			providers: [{ provide: CardIconService, useValue: { getCardIconSvg: () => '' } }]
		}).compileComponents();

		fixture = TestBed.createComponent(BillingOverviewComponent);
		component = fixture.componentInstance;
	});

	function overview(overrides: Partial<BillingOverview> = {}): BillingOverview {
		return {
			plan: { name: 'Cloud Pro', price: '$199' },
			usage: { period: 'Jan 01 - Feb 01', resetIn: '5 days' },
			payment: null,
			...overrides
		};
	}

	function text(): string {
		return (fixture.nativeElement as HTMLElement).textContent || '';
	}

	function bannerText(): string {
		const banner = (fixture.nativeElement as HTMLElement).querySelector('.billing-card-past-due');
		return banner?.textContent || '';
	}

	it('renders the past-due banner and a single Pay now CTA when suspended', () => {
		component.overview = overview({ pastDue: true });
		component.isLoading = false;
		fixture.detectChanges();

		expect(component.isPastDue).toBeTrue();
		expect(bannerText()).toContain('Payment past due');
		expect(bannerText()).toContain('Pay now');
		// Past due is a paid-customer state (an expired no-card trial is cancelled,
		// never suspended), so the banner names the real plan, not a trial outcome.
		expect(bannerText()).toContain('Cloud Pro');
		expect(bannerText()).not.toContain('Trial ended');
		// The banner itself no longer duplicates "Add payment method" (the payment
		// details card owns that single link), and it does not show "Manage plan".
		expect(bannerText()).not.toContain('Add payment method');
		expect(bannerText()).not.toContain('Resubscribe');
		expect(text()).not.toContain('Manage plan');
	});

	it('routes the Pay now CTA to the pay-past-due (invoice) flow', () => {
		component.overview = overview({ pastDue: true });
		component.isLoading = false;
		fixture.detectChanges();

		const spy = spyOn(component.payPastDue, 'emit');
		const link = (fixture.nativeElement as HTMLElement).querySelector('.billing-card-link-primary') as HTMLAnchorElement;
		expect(link.textContent).toContain('Pay now');

		link.click();
		expect(spy).toHaveBeenCalled();
	});

	it('renders the normal plan card with Manage plan when healthy', () => {
		component.overview = overview({ pastDue: false });
		component.isLoading = false;
		fixture.detectChanges();

		expect(component.isPastDue).toBeFalse();
		expect(text()).toContain('Cloud Pro');
		expect(text()).toContain('Manage plan');
		expect(text()).not.toContain('Payment past due');
	});

	it('offers "Add payment method" during an active trial for auto-convert at trial end', () => {
		component.overview = overview({ plan: { name: 'Cloud Pro', price: '$199', trial: true }, payment: null });
		component.isLoading = false;
		fixture.detectChanges();

		expect(text()).toContain('Add payment method');
		expect(text()).toContain('Subscribe now');
	});

	it('still offers "Add payment method" to a non-trial org without a card', () => {
		component.overview = overview({ plan: { name: 'Cloud Pro', price: '$199', trial: false }, payment: null });
		component.isLoading = false;
		fixture.detectChanges();

		expect(text()).toContain('Add payment method');
	});

	it('keeps "Manage" for a card already on file during a trial', () => {
		component.overview = overview({ plan: { name: 'Cloud Pro', price: '$199', trial: true }, payment: { last4: '4242', brand: 'visa' } });
		component.isLoading = false;
		fixture.detectChanges();

		expect(text()).not.toContain('Add payment method');
		expect(text()).toContain('Manage');
	});

	it('does not show the banner while billing data is still loading', () => {
		component.overview = overview({ pastDue: true });
		component.isLoading = true;
		fixture.detectChanges();

		expect(component.isPastDue).toBeFalse();
		expect(text()).not.toContain('Payment past due');
	});

	it('does not show reset countdown while billing data is still loading', () => {
		component.overview = overview({ usage: { period: 'Jun 03 - Jul 03', resetIn: '1 day' } });
		component.isLoading = true;
		fixture.detectChanges();

		expect(text()).not.toContain('Resets in');
		expect(text()).not.toContain('days');
	});

	it('shows reset countdown after billing data has loaded', () => {
		component.overview = overview({ usage: { period: 'Jun 03 - Jul 03', resetIn: '1 day' } });
		component.isLoading = false;
		fixture.detectChanges();

		expect(text()).toContain('Resets in 1 day');
	});
});
