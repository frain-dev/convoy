import {ChangeDetectorRef, Component, DestroyRef, ElementRef, HostListener, OnInit, ViewChild, inject} from '@angular/core';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {finalize} from 'rxjs/operators';
import {ActivatedRoute, Router} from '@angular/router';
import {StripeElementsComponent} from './stripe-elements.component';
import {FormBuilder, FormGroup, Validators} from '@angular/forms';
import {
    BillingAddressDetails,
    BillingPaymentDetailsService,
    PaymentMethod,
    PaymentMethodDetails,
    VatInfoDetails
} from './billing-payment-details.service';
import {CardIconService} from './card-icon.service';
import {GeneralService} from 'src/app/services/general/general.service';
import {CountriesService} from 'src/app/services/countries/countries.service';
import {Plan, PlanFeature, PlanService} from './plan.service';
import {BillingOverviewService, BillingOverview} from './billing-overview.service';
import {BillingUsageService, UsageRow} from './billing-usage.service';
import {BillingInvoicesService} from './billing-invoices.service';
import {HttpService} from 'src/app/services/http/http.service';
import {LicensesService} from 'src/app/services/licenses/licenses.service';
import {BillingStrategy, Subscription, SelfHostedBillingConfig, TaxIdType, CHECKOUT_STATUS} from 'src/app/models/billing.model';
import {BillingEndpoints} from './billing-endpoints';
import {PlanCatalogService} from './plan-catalog.service';
import {BillingLocationService} from './billing-location.service';
import {vatNumberValidator} from './vat-number.validator';
import {zipCodeValidator} from './zip-code.validator';

// Delay before reading payment details after a save, allowing the provider
// webhook to be processed by the billing service.
const WEBHOOK_PROCESSING_DELAY_MS = 1500;
// Brief pause so the loading state renders before the provider confirm call.
const PAYMENT_SUBMIT_DELAY_MS = 100;
const PAYMENT_DETAILS_MAX_RETRIES = 5;
const PAYMENT_DETAILS_RETRY_DELAY_MS = 1000;
const SUPPORT_EMAIL = 'support@getconvoy.io';
// Cloud usage is computed in the background; while the API reports pending,
// poll a bounded number of times so the placeholder is replaced once the
// figure is ready, then give up (the backend logs persistent failures). The
// window (~2 min) matches the backend recompute lock TTL so a slow first
// aggregation still resolves before polling stops.
// Poll past the server's ~2 minute recompute lock so a figure cached right at
// the lock boundary is still picked up before the page gives up and leaves the
// placeholder for a manual reload. 30 x 5s = 150s.
const USAGE_PENDING_MAX_POLLS = 30;
const USAGE_PENDING_POLL_DELAY_MS = 5000;

@Component({
    selector: 'app-billing-page',
    templateUrl: './billing-page.component.html',
    styleUrls: ['./billing-page.component.scss'],
    standalone: false
})
export class BillingPageComponent implements OnInit {
  @ViewChild('paymentDetailsDialog') paymentDetailsDialog!: ElementRef<HTMLDialogElement>;
  @ViewChild('managePlanDialog') managePlanDialog!: ElementRef<HTMLDialogElement>;
  @ViewChild('cancelConfirmDialog') cancelConfirmDialog!: ElementRef<HTMLDialogElement>;

  isCancelConfirmOpen = false;
  refreshOverviewTrigger = 0;
  selectedPlan: string | null = null;
  currentYear = new Date().getFullYear() - 2000; // 2-digit current year
  currentMonth = new Date().getMonth() + 1; // Current month (1-12)

  plans: Plan[] = [];
  isLoadingPlans = false;
  hasAttemptedPlansLoad = false;
  hasLoadedPlans = false;
  plansUnavailableMessage = '';
  currentSubscription: Subscription | null = null;
  overwatchPlans: Plan[] = [];

  // Existing data
  paymentMethodDetails: PaymentMethodDetails | null = null;
  paymentMethods: PaymentMethod[] = [];
  billingAddressDetails: BillingAddressDetails | null = null;
  vatInfoDetails: VatInfoDetails | null = null;

  // Edit states
  isEditingPaymentMethod = false;
  isEditingBillingAddress = false;
  isEditingVat = false;

  // Loading states
  isLoadingPaymentMethod = false;
  isLoadingBillingAddress = false;
  isLoadingVat = false;
  isLoadingCheckout = false;

  selfHostedCheckoutForm!: FormGroup;
  selfHostedBillingConfig: SelfHostedBillingConfig | null = null;
  billingStrategy: BillingStrategy = 'oss';
  billingAddressForm!: FormGroup;
  vatForm!: FormGroup;

  countries: { code: string; name: string }[] = [];
  vatCountries: { code: string; name: string }[] = []; // Countries with tax ID types from billing service
  taxIdTypes: TaxIdType[] = []; // Store tax ID types with examples
  vatPlaceholder = 'Enter VAT number'; // Dynamic placeholder based on selected country
  states: string[] = [];
  cities: string[] = [];
  isLoadingCountries = false;
  isLoadingVatCountries = false;
  isLoadingStates = false;
  isLoadingCities = false;

  // API error message
  apiError = '';


  // Payment provider properties
  paymentProviderType = '';
  paymentProviderPublishableKey = '';
  setupIntentSecret = '';
  isPaymentProviderLoading = false;
  isSavingPaymentMethod = false;
  isSavingBillingAddress = false;
  internalOrganisationId = ''; // Internal ID from billing service

  // Confirmation states
  confirmingDefaultFor: string | null = null;
  confirmingDeleteFor: string | null = null;

  // Billing data for child components
  billingOverview: BillingOverview | null = null;
  usageRows: UsageRow[] = [];
  isLoadingBillingData = true;
  isLoadingUsage = false;

  constructor(
    private fb: FormBuilder,
    private billingPaymentDetailsService: BillingPaymentDetailsService,
    private generalService: GeneralService,
    private cardIconService: CardIconService,
    private countriesService: CountriesService,
    private planService: PlanService,
    private cdr: ChangeDetectorRef,
    private overviewService: BillingOverviewService,
    private invoicesService: BillingInvoicesService,
    private usageService: BillingUsageService,
    private httpService: HttpService,
    private licensesService: LicensesService,
    private planCatalog: PlanCatalogService,
    private billingLocationService: BillingLocationService,
    private route: ActivatedRoute,
    private router: Router
  ) {
    this.initializeForms();
  }

  private destroyRef = inject(DestroyRef);
  private bootstrapSubscriptionPromise: Promise<void> | null = null;
  private locationRequestToken = 0;
  private activeCountryRequestToken = 0;
  private activeCityRequestToken = 0;
  private cityLoadingRequestToken: number | null = null;
  private usagePollHandle: ReturnType<typeof setTimeout> | null = null;
  private usageRequestToken = 0;

  async ngOnInit() {
    this.destroyRef.onDestroy(() => this.clearUsagePoll());

    // When the post-checkout poll confirms the subscription is active, reload
    // billing data so the plan card and Manage plan reflect it immediately.
    this.billingPaymentDetailsService.checkoutSubscriptionVerified$
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(() => {
        if (this.canShowBillingPanels) {
          this.loadBillingData();
        }
      });

    this.validateOrganisation();
    this.loadCountries();
    await this.loadBillingConfiguration();
    if (this.billingStrategy === 'oss' && this.hasActiveSelfHostedCheckout) {
      await this.checkActiveSelfHostedCheckout(false);
    }

    if (this.billingStrategy === 'cloud') {
      // Start bootstrap in background - code that needs it will await the promise
      this.bootstrapSubscriptionPromise = this.bootstrapOrganisation();
      this.overviewService.setBootstrapPromise(this.bootstrapSubscriptionPromise);
    } else if (this.billingStrategy === 'licensed_self_hosted') {
      this.bootstrapSubscriptionPromise = null;
      this.overviewService.setBootstrapPromise(null);
      await this.loadBillingData();
    } else {
      this.bootstrapSubscriptionPromise = null;
      this.overviewService.setBootstrapPromise(null);
      this.markBillingDataIdle();
    }

    this.billingAddressForm.get('country')?.valueChanges
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(countryCode => {
        this.onCountryChange(countryCode);
      });
    this.billingAddressForm.get('state')?.valueChanges
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(stateName => {
        this.onStateChange(stateName);
      });

    this.vatForm.get('country')?.valueChanges
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(countryCode => {
        this.onVatCountryChange(countryCode);
      });

  }

  get isSelfHostedBilling(): boolean {
    return this.billingStrategy !== 'cloud';
  }

  get canShowBillingPanels(): boolean {
    return this.billingStrategy === 'cloud' || this.billingStrategy === 'licensed_self_hosted';
  }

  get selfHostedSetupTitle(): string {
    return this.billingStrategy === 'licensed_self_hosted' ? 'Manage self-hosted billing' : 'Start self-hosted billing';
  }

  get selfHostedSetupMessage(): string {
    if (this.billingStrategy === 'licensed_self_hosted') {
      return 'Manage your plan and payment details for this self-hosted instance.';
    }
    return 'Choose a plan and enter a billing email to start checkout for this self-hosted instance.';
  }

  get hasActiveSelfHostedCheckout(): boolean {
    return !!this.selfHostedBillingConfig?.active_checkout?.attempt_id;
  }

  // Server-resolved flag, so the label matches whether checkout actually resubscribes.
  get isSelfHostedResubscribe(): boolean {
    return this.selfHostedBillingConfig?.resubscribe === true;
  }

  private async bootstrapOrganisation() {
    try {
      const orgId = this.getOrganisationId();
      await this.httpService.request({
        url: `/billing/organisations/${orgId}/subscription`,
        method: 'get',
        hideNotification: true
      });
      await this.loadBillingData();
    } catch (error) {
      console.error('Failed to bootstrap organisation:', error);
      await this.loadBillingData();
    }
  }

  private async loadBillingData() {
    if (!this.canShowBillingPanels) {
      this.markBillingDataIdle();
      return;
    }

    this.isLoadingBillingData = true;
    this.isLoadingUsage = true;
    try {
      const orgId = this.getOrganisationId();
      const subscriptionUrl = BillingEndpoints.billingUrl(this.billingStrategy, 'subscription', orgId);
      const paymentMethodsUrl = BillingEndpoints.billingUrl(this.billingStrategy, 'payment_methods', orgId);
      const paymentResponse = await this.httpService
        .request({
          url: paymentMethodsUrl,
          method: 'get',
          hideNotification: true
        })
        .catch(() => ({ data: null }));

      const subscriptionResponse = await this.httpService
        .request({
          url: subscriptionUrl,
          method: 'get',
          hideNotification: true
        })
        .catch(() => ({ data: null }));

      const hadSubscription = this.hasActiveSubscription(this.currentSubscription);
      const hasSubscription = this.hasActiveSubscription(subscriptionResponse.data);
      if (hadSubscription !== hasSubscription) {
        // Fail open: a license refresh failure here must not block rendering
        // billing data. Licenses are refreshed again on the next load.
        this.licensesService.loadAllLicenses().catch(() => {});
      }

      const overviewData = {
        subscription: subscriptionResponse.data,
        usage: null as any,
        payment: paymentResponse.data
      };

      this.currentSubscription = subscriptionResponse.data;

      this.billingOverview = this.overviewService.formatOverviewData(overviewData);
      this.usageRows = [];

      if (overviewData.payment && Array.isArray(overviewData.payment)) {
        this.paymentMethods = overviewData.payment.sort((a: PaymentMethod, b: PaymentMethod) => a.id.localeCompare(b.id));
        if (this.paymentMethods.length > 0) {
          const pm = this.paymentMethods[0];
          this.paymentMethodDetails = {
            cardholderName: 'Cardholder Name',
            last4: pm.last4 || '0000',
            brand: pm.card_type || 'unknown',
            expiryMonth: pm.exp_month?.toString() || '',
            expiryYear: pm.exp_year?.toString() || ''
          };
        }
      } else {
        this.paymentMethods = [];
      }

      this.isLoadingBillingData = false;
      this.loadUsageSeparately();
      if (this.billingStrategy === 'cloud' || this.billingStrategy === 'licensed_self_hosted') {
        await this.loadOrganisationData();
      } else {
        this.isLoadingBillingAddress = false;
        this.isLoadingVat = false;
      }
    } catch (error) {
      console.error('Failed to load billing data:', error);
      this.isLoadingBillingData = false;
      this.isLoadingUsage = false;
    }
  }

  private loadUsageSeparately(attempt = 0, token?: number) {
    if (this.billingStrategy !== 'cloud' && this.billingStrategy !== 'licensed_self_hosted') {
      this.clearUsagePoll();
      this.isLoadingUsage = false;
      this.usageRows = [];
      return;
    }

    // A fresh load (attempt 0) cancels any in-flight poll chain and mints a new
    // request token so a late response from a superseded chain is ignored.
    if (attempt === 0) {
      this.clearUsagePoll();
      token = ++this.usageRequestToken;
    }
    const requestToken = token!;

    const orgId = this.getOrganisationId();
    // Self-hosted usage is local instance data; cloud usage comes from the provider.
    // For cloud, scope the query to the billing cycle the UI shows so the figures
    // match the displayed period (backend defaults to the calendar month).
    let usageUrl: string;
    if (this.billingStrategy === 'licensed_self_hosted') {
      usageUrl = `/billing/sh_usage?orgID=${orgId}`;
    } else {
      const { start, end } = this.usageRange();
      usageUrl = `/billing/organisations/${orgId}/usage?start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}`;
    }
    this.httpService
      .request({
        url: usageUrl,
        method: 'get',
        hideNotification: true
      })
      .then(res => {
        // Drop stale responses: a newer load has superseded this chain.
        if (requestToken !== this.usageRequestToken) {
          return;
        }

        const data = res?.data;
        if (!data) {
          this.usageRows = [];
          return;
        }

        this.usageRows = this.usageService.formatUsageData(data);
        // Cloud usage may still be computing; re-poll a bounded number of times
        // so the placeholder is replaced once the real figure is cached.
        if (data.pending && attempt < USAGE_PENDING_MAX_POLLS) {
          this.usagePollHandle = setTimeout(
            () => this.loadUsageSeparately(attempt + 1, requestToken),
            USAGE_PENDING_POLL_DELAY_MS
          );
        }
      })
      .catch(() => {
        if (requestToken !== this.usageRequestToken) {
          return;
        }
        this.usageRows = [];
      })
      .finally(() => {
        if (requestToken !== this.usageRequestToken) {
          return;
        }
        this.isLoadingUsage = false;
        this.cdr.detectChanges();
      });
  }

  // Resolves the usage window to the active subscription billing cycle, falling
  // back to the current calendar month when no valid cycle is available. Bounds
  // are ISO 8601 so they round-trip through the backend's RFC3339 parsing.
  private usageRange(): { start: string; end: string } {
    const cycleStart = this.currentSubscription?.current_period_start;
    const cycleEnd = this.currentSubscription?.current_period_end;
    if (cycleStart && cycleEnd) {
      const s = new Date(cycleStart);
      const e = new Date(cycleEnd);
      if (!isNaN(s.getTime()) && !isNaN(e.getTime()) && s < e) {
        return { start: s.toISOString(), end: e.toISOString() };
      }
    }

    const now = new Date();
    const start = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1, 0, 0, 0));
    const end = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() + 1, 1, 0, 0, 0) - 1);
    return { start: start.toISOString(), end: end.toISOString() };
  }

  private clearUsagePoll() {
    if (this.usagePollHandle) {
      clearTimeout(this.usagePollHandle);
      this.usagePollHandle = null;
    }
    // Invalidate any in-flight usage response so a late resolve from a cancelled
    // chain cannot repopulate rows after idle/destroy.
    this.usageRequestToken++;
  }

  private async loadOrganisationData() {
    if (this.billingStrategy === 'licensed_self_hosted') {
      this.loadExistingData();
      return;
    }

    if (!this.canShowBillingPanels) {
      this.loadExistingData();
      return;
    }

    if (this.bootstrapSubscriptionPromise) {
      await this.bootstrapSubscriptionPromise;
    }

    this.isLoadingBillingAddress = true;
    this.isLoadingVat = true;
    try {
      const orgId = this.getOrganisationId();
      const response = await this.httpService.request({
        url: `/billing/organisations/${orgId}`,
        method: 'get',
        hideNotification: true
      }).catch(() => ({ data: null }));

      if (response.data) {
        // Load billing address
        if (response.data.billing_address) {
          this.billingAddressDetails = response.data.billing_address;
          this.isLoadingBillingAddress = false;
        } else {
          // Fallback to organisation API
          this.billingPaymentDetailsService.getBillingAddress().subscribe({
            next: (details) => {
              this.billingAddressDetails = details;
              this.isLoadingBillingAddress = false;
            },
            error: (error) => {
              console.error('Failed to load billing address:', error);
              this.billingAddressDetails = null;
              this.isLoadingBillingAddress = false;
            }
          });
        }

        // Load VAT info
        if (response.data.vat_info) {
          this.vatInfoDetails = response.data.vat_info;
          this.isLoadingVat = false;
        } else {
          // Fallback to organisation API
          this.billingPaymentDetailsService.getVatInfo().subscribe({
            next: (details) => {
              this.vatInfoDetails = details;
              this.isLoadingVat = false;
            },
            error: (error) => {
              console.error('Failed to load VAT info:', error);
              this.isLoadingVat = false;
            }
          });
        }
      } else {
        // Fallback to organisation API if billing service doesn't have data
        this.loadExistingData();
      }
    } catch (error) {
      console.error('Failed to load organisation data:', error);
      // Fallback to organisation API
      this.loadExistingData();
    }
  }

  private validateOrganisation() {
    // Surface invalid org data with a notification but do not halt; callers
    // proceed regardless (behavior preserved). The org id is validated again
    // fail-closed at each billing request via getOrganisationIdOrThrow().
    try {
      this.httpService.getOrganisationIdOrThrow();
    } catch {
      this.generalService.showNotification({
        message: 'Invalid organisation data. Please refresh the page and try again.',
        style: 'error'
      });
    }
  }

  private loadBillingConfiguration(): Promise<void> {
    return new Promise(resolve => {
      this.billingPaymentDetailsService.getBillingConfig().subscribe({
        next: (config) => {
          this.paymentProviderType = config.data.payment_provider.type;
          this.paymentProviderPublishableKey = config.data.payment_provider.publishable_key;
          this.billingStrategy = config.data.strategy || 'oss';
          this.overviewService.setBillingStrategy(this.billingStrategy);
          this.invoicesService.setBillingStrategy(this.billingStrategy);
          this.billingPaymentDetailsService.setBillingStrategy(this.billingStrategy);
          this.selfHostedBillingConfig = config.data.self_hosted || null;
          if (this.billingStrategy === 'cloud') {
            this.loadInternalOrganisationId();
          }
          resolve();
        },
        error: (error) => {
          console.error('Failed to load billing configuration:', error);
          this.generalService.showNotification({
            message: 'Failed to load billing configuration. Please refresh the page.',
            style: 'error'
          });
          this.billingStrategy = 'oss';
          this.markBillingDataIdle();
          resolve();
        }
      });
    });
  }

  private markBillingDataIdle() {
    this.clearUsagePoll();
    this.isLoadingBillingData = false;
    this.isLoadingUsage = false;
    this.billingOverview = null;
    this.usageRows = [];
    this.paymentMethods = [];
    this.paymentMethodDetails = null;
    this.isLoadingBillingAddress = false;
    this.isLoadingVat = false;
  }

  private loadInternalOrganisationId() {
    const externalOrgId = this.getOrganisationId();
    this.billingPaymentDetailsService.getInternalOrganisationId(externalOrgId).subscribe({
      next: (response) => {
        this.internalOrganisationId = response.data.id;
        this.loadExistingData();
      },
      error: (error) => {
        console.error('Failed to load internal organisation ID:', error);
        const errorMessage = this.generalService.extractMessage(error, 'Failed to load organisation data');
        this.generalService.showNotification({
          message: errorMessage,
          style: 'error'
        });
        this.internalOrganisationId = '';
        // Don't load existing data if the first call failed
      }
    });
  }

  private loadCountries() {
    this.isLoadingCountries = true;
    this.countriesService.getCountries().subscribe({
      next: (countries) => {
        this.countries = countries;
        this.isLoadingCountries = false;
        this.loadVatCountries();
      },
      error: (error) => {
        console.error('Failed to load countries:', error);
        this.isLoadingCountries = false;
        this.countries = [];
      }
    });
  }

  private loadVatCountries() {
    this.isLoadingVatCountries = true;
    this.billingPaymentDetailsService.getTaxIDTypes().subscribe({
      next: (response) => {
        const taxIdTypes = response.data || [];
        this.taxIdTypes = taxIdTypes; // Store tax ID types with examples
        this.vatCountries = [];

        taxIdTypes.forEach((taxType: any) => {
          const type = taxType.type;
          if (type) {
            const countryCode = type.split('_')[0];
            if (countryCode) {
              const country = this.countries.find(c => c.code.toLowerCase() === countryCode.toLowerCase());
              if (country && !this.vatCountries.find(vc => vc.code === country.code)) {
                this.vatCountries.push(country);
              }
            }
          }
        });

        this.isLoadingVatCountries = false;
      },
      error: (error) => {
        console.error('Failed to load VAT countries:', error);
        this.isLoadingVatCountries = false;
        this.vatCountries = [];
        this.taxIdTypes = [];
      }
    });
  }

  onVatCountryChange(countryCode: string) {
    if (!countryCode) {
      this.vatPlaceholder = 'Enter VAT number';
      return;
    }

    // Find the tax ID type for the selected country
    const countryCodeLower = countryCode.toLowerCase();
    const taxIdType = this.taxIdTypes.find((taxType: any) => {
      const type = taxType.type;
      if (type) {
        const typeCountryCode = type.split('_')[0].toLowerCase();
        return typeCountryCode === countryCodeLower;
      }
      return false;
    });

    // Set placeholder to the example if found, otherwise default
    if (taxIdType && taxIdType.example) {
      this.vatPlaceholder = taxIdType.example;
    } else {
      this.vatPlaceholder = 'Enter VAT number';
    }
  }

  onCountryChange(countryCode: string, preferredState: string = '', preferredCity: string = '') {
    const requestToken = ++this.locationRequestToken;
    this.activeCountryRequestToken = requestToken;
    this.activeCityRequestToken = requestToken;
    const isRehydration = !!preferredState || !!preferredCity;

    if (!countryCode) {
      this.states = [];
      this.cities = [];
      this.isLoadingStates = false;
      this.isLoadingCities = false;
      this.billingAddressForm.get('state')?.setValue('', { emitEvent: false });
      this.billingAddressForm.get('city')?.setValue('', { emitEvent: false });
      this.updateStateControlValidation();
      this.updateCityControlValidation();
      return;
    }

    const previousState = isRehydration ? preferredState : '';
    const previousCity = isRehydration ? preferredCity : '';
    this.billingAddressForm.get('state')?.setValue('', { emitEvent: false });
    this.billingAddressForm.get('city')?.setValue('', { emitEvent: false });
    this.states = [];
    this.cities = [];
    this.isLoadingCities = false;
    this.updateCityControlValidation();

    const countryName = this.getCountryName(countryCode);
    this.isLoadingStates = true;
    this.countriesService.getStatesForCountry(countryName).subscribe({
      next: (states) => {
        if (this.activeCountryRequestToken !== requestToken) {
          return;
        }

        this.states = states;
        this.updateStateControlValidation();
        if (this.states.length > 0) {
          const matchedState = this.states.find(state => state.trim().toLowerCase() === (previousState || '').trim().toLowerCase());
          if (matchedState) {
            this.billingAddressForm.get('state')?.setValue(matchedState, { emitEvent: false });
            this.activeCityRequestToken = requestToken;
            this.loadCitiesByState(countryName, matchedState, previousCity, requestToken);
          } else if (previousCity) {
            // Legacy records may only have city; keep options visible until user chooses state.
            this.activeCityRequestToken = requestToken;
            this.loadCitiesByCountry(countryName, previousCity, requestToken);
          } else {
            this.isLoadingCities = false;
          }
        } else {
          this.activeCityRequestToken = requestToken;
          this.loadCitiesByCountry(countryName, previousCity, requestToken);
        }
        this.isLoadingStates = false;
      },
      error: (error) => {
        if (this.activeCountryRequestToken !== requestToken) {
          return;
        }

        console.error('Failed to load states:', error);
        this.states = [];
        this.updateStateControlValidation();
        this.activeCityRequestToken = requestToken;
        this.loadCitiesByCountry(countryName, previousCity, requestToken);
        this.isLoadingStates = false;
      }
    });
  }

  onStateChange(stateName: string) {
    const countryCode = this.billingAddressForm.get('country')?.value;
    if (!countryCode) {
      return;
    }

    const countryName = this.getCountryName(countryCode);
    if (!stateName) {
      if (this.states.length > 0) {
        ++this.locationRequestToken;
        this.activeCityRequestToken = this.locationRequestToken;
        this.cities = [];
        this.isLoadingCities = false;
        this.billingAddressForm.get('city')?.setValue('', { emitEvent: false });
        this.updateCityControlValidation();
        return;
      }

      const requestToken = ++this.locationRequestToken;
      this.activeCityRequestToken = requestToken;
      this.loadCitiesByCountry(countryName, '', requestToken);
      return;
    }

    // On user state changes, do not preserve previous city value
    // to avoid invalid state/city combinations being silently carried over.
    const requestToken = ++this.locationRequestToken;
    this.activeCityRequestToken = requestToken;
    this.loadCitiesByState(countryName, stateName, '', requestToken);
  }

  private loadCitiesByCountry(countryName: string, preferredCity: string = '', requestToken: number = this.activeCityRequestToken) {
    this.isLoadingCities = true;
    this.cityLoadingRequestToken = requestToken;
    this.countriesService.getCitiesForCountry(countryName).subscribe({
      next: (cities) => {
        if (this.activeCityRequestToken !== requestToken) {
          if (this.cityLoadingRequestToken === requestToken) {
            this.isLoadingCities = false;
            this.cityLoadingRequestToken = null;
          }
          return;
        }

        this.cities = this.withPreferredCity(cities, preferredCity);
        const matchedCity = this.findMatchingCity(this.cities, preferredCity);
        if (matchedCity) {
          this.billingAddressForm.get('city')?.setValue(matchedCity, { emitEvent: false });
        } else {
          this.billingAddressForm.get('city')?.setValue('', { emitEvent: false });
        }
        this.updateCityControlValidation();
        this.isLoadingCities = false;
        this.cityLoadingRequestToken = null;
      },
      error: (error) => {
        if (this.activeCityRequestToken !== requestToken) {
          if (this.cityLoadingRequestToken === requestToken) {
            this.isLoadingCities = false;
            this.cityLoadingRequestToken = null;
          }
          return;
        }

        console.error('Failed to load cities:', error);
        this.isLoadingCities = false;
        this.cityLoadingRequestToken = null;
        this.cities = [];
        this.billingAddressForm.get('city')?.setValue('', { emitEvent: false });
        this.updateCityControlValidation();
      }
    });
  }

  private loadCitiesByState(countryName: string, stateName: string, preferredCity: string = '', requestToken: number = this.activeCityRequestToken) {
    this.isLoadingCities = true;
    this.cityLoadingRequestToken = requestToken;
    this.countriesService.getCitiesForCountryAndState(countryName, stateName).subscribe({
      next: (cities) => {
        if (this.activeCityRequestToken !== requestToken) {
          if (this.cityLoadingRequestToken === requestToken) {
            this.isLoadingCities = false;
            this.cityLoadingRequestToken = null;
          }
          return;
        }

        if (!cities || cities.length === 0) {
          this.loadCitiesByCountry(countryName, preferredCity, requestToken);
          return;
        }

        this.cities = this.withPreferredCity(cities, preferredCity);
        const matchedCity = this.findMatchingCity(this.cities, preferredCity);
        if (matchedCity) {
          this.billingAddressForm.get('city')?.setValue(matchedCity, { emitEvent: false });
        } else {
          this.billingAddressForm.get('city')?.setValue('', { emitEvent: false });
        }
        this.updateCityControlValidation();
        this.isLoadingCities = false;
        this.cityLoadingRequestToken = null;
      },
      error: (error) => {
        if (this.activeCityRequestToken !== requestToken) {
          if (this.cityLoadingRequestToken === requestToken) {
            this.isLoadingCities = false;
            this.cityLoadingRequestToken = null;
          }
          return;
        }

        console.error('Failed to load cities by state:', error);
        this.loadCitiesByCountry(countryName, preferredCity, requestToken);
      }
    });
  }

  private initializeForms() {
    this.selfHostedCheckoutForm = this.fb.group({
      email: ['', [Validators.required, Validators.email]]
    });

    this.billingAddressForm = this.fb.group({
      name: ['', [Validators.required, Validators.minLength(2), Validators.maxLength(100)]],
      addressLine1: ['', [Validators.required, Validators.minLength(5), Validators.maxLength(200)]],
      addressLine2: ['', [Validators.maxLength(200)]],
      country: ['', Validators.required],
      state: [''],
      city: [''],
      zipCode: ['', [Validators.required, Validators.minLength(3), Validators.maxLength(20), zipCodeValidator()]]
    });

    this.vatForm = this.fb.group({
      businessName: ['', [Validators.required, Validators.minLength(2), Validators.maxLength(200)]],
      country: ['', Validators.required],
      vatNumber: ['', [Validators.required, vatNumberValidator()]]
    });
  }

  openPaymentDetails() {
    if (!this.canShowBillingPanels) return;
    this.paymentDetailsDialog.nativeElement.showModal();
    this.loadExistingData();
  }

  closePaymentDetails() {
    this.paymentDetailsDialog.nativeElement.close();
    this.resetEditStates();
    this.resetForms();
  }

  openManagePlan() {
    this.selectedPlan = null; // Reset selection when opening dialog
    this.loadPlans();
    this.managePlanDialog.nativeElement.showModal();
  }

  async resumeSelfHostedCheckout() {
    await this.checkActiveSelfHostedCheckout(true);
  }

  private async checkActiveSelfHostedCheckout(redirectOnPending: boolean) {
    const activeCheckout = this.selfHostedBillingConfig?.active_checkout;
    if (!activeCheckout?.attempt_id) {
      if (redirectOnPending) {
        this.openManagePlan();
      }
      return;
    }

    this.isLoadingCheckout = true;
    try {
      const response = await this.httpService.request({
        url: '/billing/sh_checkout/complete',
        method: 'post',
        body: {
          attempt_id: activeCheckout.attempt_id,
          checkout_id: activeCheckout.checkout_id
        },
        hideNotification: true
      });

      if (response.data?.status === CHECKOUT_STATUS.COMPLETED) {
        await this.licensesService.loadAllLicenses();
        this.generalService.showNotification({ message: 'License activated successfully!', style: 'success' });
        await this.loadBillingConfiguration();
        return;
      }

      if (redirectOnPending && response.data?.status === CHECKOUT_STATUS.PENDING && activeCheckout.checkout_url) {
        window.location.href = activeCheckout.checkout_url;
        return;
      }

      if (!redirectOnPending && response.data?.status === CHECKOUT_STATUS.PENDING) {
        return;
      }

      this.generalService.showNotification({
        message: response.data?.status === CHECKOUT_STATUS.EXPIRED ? 'Checkout has expired. Start a new checkout to continue.' : 'Checkout is still waiting for payment. Start a new checkout if the payment page is no longer available.',
        style: 'warning'
      });
    } catch (error: any) {
      this.generalService.showNotification({
        message: error?.error?.message || 'Unable to resume checkout. Please try again.',
        style: 'error'
      });
    } finally {
      this.isLoadingCheckout = false;
    }
  }

  private loadPlans() {
    this.isLoadingPlans = true;
    this.hasAttemptedPlansLoad = true;
    this.hasLoadedPlans = false;
    this.plansUnavailableMessage = '';

    this.planService.getPlans().subscribe({
      next: (response) => {
        const defaultData = this.planService.getDefaultPlanComparison();
        const plansFromApi = Array.isArray(response.data) ? response.data : [];

        const catalog = this.planCatalog.buildCatalog(plansFromApi, defaultData.plans, this.isSelfHostedBilling);
        this.plans = catalog.plans;
        this.overwatchPlans = catalog.overwatchPlans;
        this.plansUnavailableMessage = catalog.plansUnavailableMessage;
        this.hasLoadedPlans = this.plans.length > 0;
        if (this.selectedPlan && !this.plans.some(plan => plan.id === this.selectedPlan)) {
          this.selectedPlan = null;
        }
        this.isLoadingPlans = false;
      },
      error: (error) => {
        console.warn('Failed to load plans from backend:', error);
        this.plans = [];
        this.overwatchPlans = [];
        this.selectedPlan = null;
        this.plansUnavailableMessage = 'Plans could not be loaded. Please try again later.';
        this.isLoadingPlans = false;
      }
    });
  }

  closeManagePlan() {
    this.managePlanDialog.nativeElement.close();
  }

  isCancellingSubscription = false;

  onCancelPlan() {
    if (!this.currentSubscription || !this.currentSubscription.id) {
      this.generalService.showNotification({
        message: 'No active subscription found',
        style: 'error'
      });
      return;
    }

    this.openCancelConfirm();
  }

  openCancelConfirm() {
    this.isCancelConfirmOpen = true;
    this.cancelConfirmDialog.nativeElement.showModal();
  }

  closeCancelConfirm() {
    this.isCancelConfirmOpen = false;
    this.cancelConfirmDialog.nativeElement.close();
  }

  async confirmCancelSubscription() {
    this.closeCancelConfirm();

    this.isCancellingSubscription = true;
    try {
      const orgId = this.getOrganisationId();
      const subscriptionId = this.currentSubscription?.id;
      const url = this.billingStrategy === 'licensed_self_hosted'
        ? '/billing/sh_subscription'
        : `/billing/organisations/${orgId}/subscriptions/${subscriptionId}`;

      await this.httpService.request({
        url,
        method: 'delete'
      });

      await this.licensesService.loadAllLicenses();
      this.closeManagePlan();

      this.generalService.showNotification({
        message: 'Subscription cancelled successfully',
        style: 'success'
      });

      await this.loadBillingData();
    } catch (error: any) {
      console.error('Failed to cancel subscription:', error);
      this.generalService.showNotification({
        message: error?.error?.message || 'Failed to cancel subscription. Please try again.',
        style: 'error'
      });
    } finally {
      this.isCancellingSubscription = false;
    }
  }

  async onUpgradePlan() {
    if (!this.hasPlanReadyState) {
      this.generalService.showNotification({
        message: this.plansUnavailableMessage || 'Plans are not available right now',
        style: 'error'
      });
      return;
    }

    if (!this.selectedPlan) {
      this.generalService.showNotification({
        message: 'Please select a plan first',
        style: 'error'
      });
      return;
    }

    const selectedPlanData = this.plans.find(p => p.id === this.selectedPlan);
    if (!selectedPlanData) {
      this.generalService.showNotification({
        message: 'Selected plan not found',
        style: 'error'
      });
      return;
    }

    const { planExistsInOverwatch, planIdForApi } = this.planCatalog.resolvePlanForApi(selectedPlanData, this.overwatchPlans);
    if (!this.canUsePlan(selectedPlanData)) {
      this.generalService.showNotification({
        message: 'This plan is not available for checkout right now',
        style: 'error'
      });
      return;
    }

    if (this.isCurrentSubscriptionPlan(planIdForApi, selectedPlanData.name)) {
      this.generalService.showNotification({
        message: 'You are already on this plan',
        style: 'success'
      });
      return;
    }

    if (this.planRequiresContact(selectedPlanData) || this.planCatalog.shouldContactForMissingCloudPlan(selectedPlanData, this.isSelfHostedBilling, planExistsInOverwatch)) {
      this.openPlanContact(selectedPlanData);
      return;
    }

    this.isLoadingCheckout = true;
    try {
      const orgId = this.getOrganisationId();
      const host = window.location.origin;

      let checkoutUrl: string;

      if (this.isSelfHostedBilling || this.planCatalog.isSelfHostedPlan(selectedPlanData)) {
        // Resubscribe reuses the known org/customer by license key, so email is omitted.
        if (!this.isSelfHostedResubscribe && !this.selfHostedCheckoutForm.valid) {
          this.selfHostedCheckoutForm.markAllAsTouched();
          this.isLoadingCheckout = false;
          return;
        }

        const body: { plan_id: string; interval: string; host: string; email?: string } = {
          plan_id: planIdForApi,
          interval: this.planCatalog.resolveCheckoutCadence(selectedPlanData),
          host: host
        };
        if (!this.isSelfHostedResubscribe) {
          body.email = this.selfHostedCheckoutForm.value.email;
        }

        const response = await this.httpService.request({
          url: '/billing/sh_checkout/start',
          method: 'post',
          body
        });

        if (response.data && response.data.checkout_url) {
          window.location.href = response.data.checkout_url;
          return;
        }

        throw new Error('Checkout URL not found in response');
      }

      if (this.currentSubscription && this.currentSubscription.id) {
        const response = await this.httpService.request({
          url: `/billing/organisations/${orgId}/subscriptions/${this.currentSubscription.id}/upgrade`,
          method: 'put',
          body: {
            plan_id: planIdForApi,
            host: host
          }
        });

        if (response.data && response.data.checkout_url) {
          checkoutUrl = response.data.checkout_url;
        } else {
          throw new Error('Checkout URL not found in response');
        }
      } else {
        const response = await this.httpService.request({
          url: `/billing/organisations/${orgId}/subscriptions/onboard`,
          method: 'post',
          body: {
            plan_id: planIdForApi,
            host: host
          }
        });

        if (response.data && response.data.checkout_url) {
          checkoutUrl = response.data.checkout_url;
        } else {
          throw new Error('Checkout URL not found in response');
        }
      }

      // Open in same window since callback will redirect back
      window.location.href = checkoutUrl;
    } catch (error: any) {
      this.isLoadingCheckout = false;
      console.error('Failed to create checkout session:', error);
      this.generalService.showNotification({
        message: error?.error?.message || 'Failed to create checkout session. Please try again.',
        style: 'error'
      });
    }
  }

  isCurrentPlan(planId: string): boolean {
    if (!this.currentSubscription || !this.currentSubscription.plan) {
      return false;
    }

    const plan = this.plans.find(p => p.id === planId);
    if (!plan) return false;

    const { planIdForApi } = this.planCatalog.resolvePlanForApi(plan, this.overwatchPlans);
    return this.isCurrentSubscriptionPlan(planIdForApi, plan.name);
  }

  getButtonText(planId: string): string {
    if (!this.hasPlanReadyState) {
      return 'Unavailable';
    }

    const plan = this.plans.find(p => p.id === planId);

    if (this.isLoadingCheckout && this.selectedPlan === planId) {
      return 'Loading...';
    }
    if (this.isCurrentPlan(planId)) {
      return 'Current Plan';
    }
    if (plan && this.planRequiresContact(plan)) {
      return 'Contact Us';
    }
    if (plan && !this.canUsePlan(plan)) {
      return 'Unavailable';
    }
    if (this.selectedPlan === planId) {
      if (plan && (this.isSelfHostedBilling || this.planCatalog.isSelfHostedPlan(plan))) {
        return this.isSelfHostedResubscribe ? 'Resubscribe' : 'Start checkout';
      }
      return this.currentSubscription ? 'Upgrade' : 'Subscribe';
    }
    return 'Select';
  }

  getCancelButtonText(): string {
    return this.isCancellingSubscription ? 'Cancelling...' : 'Cancel Subscription';
  }

  selectPlan(planId: string) {
    if (!this.plans.some(plan => plan.id === planId)) {
      return;
    }

    this.selectedPlan = planId;
  }

  clearPlanSelection(): void {
    if (this.isLoadingCheckout) {
      return;
    }

    this.selectedPlan = null;
  }

  selectedPlanIsSelfHosted(): boolean {
    const plan = this.plans.find(p => p.id === this.selectedPlan);
    return !!plan && !this.planRequiresContact(plan) && (this.isSelfHostedBilling || this.planCatalog.isSelfHostedPlan(plan));
  }

  // The billing email is only needed to start a checkout. Selecting the current
  // plan is allowed for visibility, but it cannot be checked out, so hide the form.
  showSelfHostedCheckoutForm(): boolean {
    return this.selectedPlanIsSelfHosted() && !!this.selectedPlan && !this.isCurrentPlan(this.selectedPlan);
  }

  planRequiresContact(plan: Plan): boolean {
    if (plan.requires_contact !== undefined) {
      return plan.requires_contact;
    }
    if (plan.checkout_enabled !== undefined) {
      return !plan.checkout_enabled;
    }
    return this.planCatalog.isEnterprisePlan(plan);
  }

  private openPlanContact(plan: Plan): void {
    const subject = encodeURIComponent(`${plan.name} Plan Request`);
    const body = encodeURIComponent(`Hello,\n\nI would like to learn more about the ${plan.name} plan.\n\nThank you.`);
    window.location.href = `mailto:${SUPPORT_EMAIL}?subject=${subject}&body=${body}`;
  }

  /** Handle plan card button: Select selects the plan; Upgrade/Subscribe triggers checkout. */
  onPlanCardButtonClick(planId: string): void {
    if (this.isCurrentPlan(planId)) return;
    const plan = this.plans.find(p => p.id === planId);
    if (!plan || !this.canUsePlan(plan)) return;
    if (plan && this.planRequiresContact(plan)) {
      this.selectedPlan = planId;
      this.openPlanContact(plan);
      return;
    }
    if (this.selectedPlan === planId) {
      this.onUpgradePlan();
    } else {
      this.selectPlan(planId);
    }
  }

  getFeaturesByCategory(category: 'core' | 'security' | 'support'): PlanFeature[] {
    if (this.plans.length === 0) return [];

    const allFeatures = this.plans.flatMap(plan =>
      plan.features.filter(feature => feature.category === category)
    );

    const uniqueFeatures = allFeatures.filter((feature, index, self) =>
      index === self.findIndex(f => f.name === feature.name)
    );

    return uniqueFeatures;
  }

  get hasPlanLoadingState(): boolean {
    return this.isLoadingPlans;
  }

  get hasPlanUnavailableState(): boolean {
    return !this.isLoadingPlans && this.hasAttemptedPlansLoad && !this.hasLoadedPlans;
  }

  get hasPlanReadyState(): boolean {
    return !this.isLoadingPlans && this.hasLoadedPlans && this.plans.length > 0;
  }

  canUsePlan(plan: Plan): boolean {
    if (this.planRequiresContact(plan)) {
      return true;
    }

    const { planExistsInOverwatch } = this.planCatalog.resolvePlanForApi(plan, this.overwatchPlans);
    if (this.isSelfHostedBilling || this.planCatalog.isSelfHostedPlan(plan)) {
      return planExistsInOverwatch;
    }

    return true;
  }

  getFeatureValue(planId: string, featureName: string): string {
    const plan = this.plans.find(p => p.id === planId);
    if (!plan) return 'Unsupported';

    const feature = plan.features.find(f => f.name === featureName);
    return feature ? feature.value : 'Unsupported';
  }

  getFeatureValueType(planId: string, featureName: string): 'supported' | 'unsupported' | 'plain' {
    const value = this.getFeatureValue(planId, featureName);

    if (value === 'Supported') return 'supported';
    if (value === 'Unsupported') return 'unsupported';
    return 'plain';
  }

  private loadExistingData() {
    this.loadPaymentMethods();
    this.loadPaymentMethodDetails();
    this.loadBillingAddress();
    this.loadVatInfo();
  }

  private loadPaymentMethods() {
    this.isLoadingPaymentMethod = true;
    this.billingPaymentDetailsService.getPaymentMethods()
      .pipe(finalize(() => this.isLoadingPaymentMethod = false))
      .subscribe({
        next: (methods) => {
          // Sort by ID to maintain consistent order
          this.paymentMethods = methods.sort((a, b) => a.id.localeCompare(b.id));
        },
        error: (error) => {
          console.error('Failed to load payment methods:', error);
          this.paymentMethods = [];
        }
      });
  }

  private loadPaymentMethodDetails() {
    this.isLoadingPaymentMethod = true;
    this.billingPaymentDetailsService.getPaymentMethodDetails()
      .pipe(finalize(() => this.isLoadingPaymentMethod = false))
      .subscribe({
        next: (details) => {
          this.paymentMethodDetails = details;
        },
        error: (error) => {
          console.error('Failed to load payment method details:', error);
        }
      });
  }

  private loadPaymentMethodDetailsWithRetry(maxRetries: number = PAYMENT_DETAILS_MAX_RETRIES, retryDelay: number = PAYMENT_DETAILS_RETRY_DELAY_MS) {
    let retryCount = 0;

    const attemptLoad = () => {
      this.isLoadingPaymentMethod = true;
      this.billingPaymentDetailsService.getPaymentMethodDetails().subscribe({
        next: (details) => {
          // Check if payment method actually exists (has last4)
          if (details && details.last4) {
            this.paymentMethodDetails = details;
            this.isLoadingPaymentMethod = false;
          } else if (retryCount < maxRetries) {
            retryCount++;
            setTimeout(attemptLoad, retryDelay);
          } else {
            this.paymentMethodDetails = details;
            this.isLoadingPaymentMethod = false;
          }
        },
        error: () => {
          if (retryCount < maxRetries) {
            retryCount++;
            setTimeout(attemptLoad, retryDelay);
          } else {
            this.isLoadingPaymentMethod = false;
          }
        }
      });
    };

    attemptLoad();
  }

  private loadBillingAddress() {
    this.isLoadingBillingAddress = true;
    this.billingPaymentDetailsService.getBillingAddress()
      .pipe(finalize(() => this.isLoadingBillingAddress = false))
      .subscribe({
        next: (details) => {
          this.billingAddressDetails = details;
        },
        error: (error) => {
          console.error('Failed to load billing address:', error);
          this.billingAddressDetails = null; // Clear any existing data on error
        }
      });
  }

  private loadVatInfo() {
    this.isLoadingVat = true;
    this.billingPaymentDetailsService.getVatInfo()
      .pipe(finalize(() => this.isLoadingVat = false))
      .subscribe({
        next: (details) => {
          this.vatInfoDetails = details;
        },
        error: (error) => {
          console.error('Failed to load VAT info:', error);
        }
      });
  }

  // Edit mode methods
  startEditingPaymentMethod() {
    try {
      this.httpService.getOrganisationIdOrThrow();
    } catch {
      this.generalService.showNotification({
        message: 'Invalid organisation data. Please refresh the page and try again.',
        style: 'error'
      });
      return;
    }

    this.isEditingPaymentMethod = true;
    this.isPaymentProviderLoading = true;
    this.getSetupIntent();
  }


  startEditingBillingAddress() {
    this.isEditingBillingAddress = true;
    if (this.billingAddressDetails) {
      const formData = { ...this.billingAddressDetails };
      this.billingAddressForm.patchValue(formData, { emitEvent: false });
      this.onCountryChange(formData.country || '', formData.state || '', formData.city || '');
    }
  }

  startEditingVat() {
    this.isEditingVat = true;
    if (this.vatInfoDetails) {
      const formData = { ...this.vatInfoDetails };
      this.vatForm.patchValue(formData);
    }
  }

  cancelEditingPaymentMethod() {
    this.isEditingPaymentMethod = false;
    this.setupIntentSecret = '';
  }

  // Payment provider Elements methods
  getSetupIntent() {
    this.isPaymentProviderLoading = true;
    this.billingPaymentDetailsService.getSetupIntent().subscribe({
      next: (setupIntentResponse) => {
        this.setupIntentSecret = setupIntentResponse.data.intent_secret;
        this.isPaymentProviderLoading = false;
      },
      error: (_error) => {
        this.generalService.showNotification({
          message: 'Failed to initialize payment form. Please try again.',
          style: 'error'
        });
        this.isPaymentProviderLoading = false;
        this.isEditingPaymentMethod = false;
      }
    });
  }

  onPaymentMethodCreated() {
    this.generalService.showNotification({
      message: 'Payment method saved successfully!',
      style: 'success'
    });
    this.isEditingPaymentMethod = false;
    this.setupIntentSecret = '';
    this.refreshOverviewTrigger++;

    // Wait for webhook to process before loading payment method details.
    // The provider sends a webhook to the billing service which processes
    // asynchronously, so the new method is not readable immediately.
    setTimeout(() => {
      this.loadPaymentMethodDetailsWithRetry();
      this.loadPaymentMethods();
    }, WEBHOOK_PROCESSING_DELAY_MS);
  }

  setDefaultPaymentMethod(pmId: string) {
    // Cancel any existing confirmation
    if (this.confirmingDefaultFor || this.confirmingDeleteFor) {
      this.cancelSetDefault();
      this.cancelDelete();
    }
    // Show confirmation UI
    this.confirmingDefaultFor = pmId;
  }

  confirmSetDefault() {
    if (!this.confirmingDefaultFor) return;

    const pmId = this.confirmingDefaultFor;
    this.confirmingDefaultFor = null;

    this.billingPaymentDetailsService.setDefaultPaymentMethod(pmId).subscribe({
      next: () => {
        this.generalService.showNotification({
          message: 'Default payment method updated successfully!',
          style: 'success'
        });
        this.loadPaymentMethods();
        this.loadPaymentMethodDetails();
        this.refreshOverviewTrigger++;
      },
      error: (error) => {
        console.error('Failed to set default payment method:', error);
        this.generalService.showNotification({
          message: 'Failed to set default payment method. Please try again.',
          style: 'error'
        });
      }
    });
  }

  deletePaymentMethod(pmId: string) {
    // Cancel any existing confirmation
    if (this.confirmingDefaultFor || this.confirmingDeleteFor) {
      this.cancelSetDefault();
      this.cancelDelete();
    }
    // Show confirmation UI
    this.confirmingDeleteFor = pmId;
  }

  confirmDelete() {
    if (!this.confirmingDeleteFor) return;

    const pmId = this.confirmingDeleteFor;
    this.confirmingDeleteFor = null;

    this.billingPaymentDetailsService.deletePaymentMethod(pmId).subscribe({
      next: () => {
        this.generalService.showNotification({
          message: 'Payment method deleted successfully!',
          style: 'success'
        });
        this.loadPaymentMethods();
        this.loadPaymentMethodDetails();
        this.refreshOverviewTrigger++;
      },
      error: (error) => {
        console.error('Failed to delete payment method:', error);
        const errorMessage = error?.error?.message || 'Failed to delete payment method. Please try again.';
        this.generalService.showNotification({
          message: errorMessage,
          style: 'error'
        });
      }
    });
  }

  cancelSetDefault() {
    this.confirmingDefaultFor = null;
    // Reset radio buttons to match actual default
    setTimeout(() => {
      const radios = document.querySelectorAll('input[name="defaultPaymentMethod"]') as NodeListOf<HTMLInputElement>;
      radios.forEach((radio, index) => {
        const pm = this.paymentMethods[index];
        if (pm) {
          radio.checked = this.isDefaultPaymentMethod(pm);
        }
      });
    }, 0);
  }

  cancelDelete() {
    this.confirmingDeleteFor = null;
  }

  @HostListener('document:click', ['$event'])
  onDocumentClick(event: MouseEvent) {
    // Check if click is outside the confirmation UI
    const target = event.target as HTMLElement;
    const confirmationElement = target.closest('.confirmation-ui');
    const paymentMethodCard = target.closest('[data-payment-method-card]');

    // If clicking outside confirmation UI and not on a payment method action, cancel
    if (!confirmationElement && !paymentMethodCard && (this.confirmingDefaultFor || this.confirmingDeleteFor)) {
      this.cancelSetDefault();
      this.cancelDelete();
    }
  }

  isDefaultPaymentMethod(pm: PaymentMethod): boolean {
    return pm.defaulted_at !== null && pm.defaulted_at !== undefined;
  }

  getCardIconForMethod(pm: PaymentMethod) {
    return this.cardIconService.getCardIconSvg(pm.card_type);
  }

  onPaymentProviderError(errorMessage: string) {
    this.apiError = errorMessage;
  }

  getOrganisationId(): string {
    return this.httpService.getOrganisation()?.uid || '';
  }

  private hasActiveSubscription(data: Subscription | null): boolean {
    return !!(data && (data.id || (data.plan && (data.plan.id || data.plan.name))));
  }

  async onUpdatePaymentMethodWithProvider(stripeElementsComponent: StripeElementsComponent, event?: Event) {
    if (event) {
      event.preventDefault();
      event.stopPropagation();
    }

    this.isSavingPaymentMethod = true;

    await new Promise(resolve => setTimeout(resolve, PAYMENT_SUBMIT_DELAY_MS));

    try {
      const success = await stripeElementsComponent.confirmPaymentMethod();
      if (success) {
        this.onPaymentMethodCreated();
      }
    } catch (error) {
      this.generalService.showNotification({
        message: 'Failed to save payment method. Please try again.',
        style: 'error'
      });
    } finally {
      this.isSavingPaymentMethod = false;
    }
  }

  cancelEditingBillingAddress() {
    this.isEditingBillingAddress = false;
    this.billingAddressForm.reset();
  }

  cancelEditingVat() {
    this.isEditingVat = false;
    this.vatForm.reset();
  }

  private resetEditStates() {
    this.isEditingPaymentMethod = false;
    this.isEditingBillingAddress = false;
    this.isEditingVat = false;
  }

  private resetForms() {
    this.billingAddressForm.reset();
    this.vatForm.reset();
    this.apiError = '';
  }


  onUpdateBillingAddress() {
    if (this.isLoadingStates || this.isLoadingCities) {
      return;
    }

    const stateControl = this.billingAddressForm.get('state');
    const cityControl = this.billingAddressForm.get('city');
    if (this.states.length > 0 && (!stateControl || !stateControl.value || !this.states.includes(stateControl.value))) {
      stateControl?.setErrors({ required: true });
      this.markFormGroupTouched(this.billingAddressForm);
      return;
    }

    if (this.cities.length > 0 && (!cityControl || !cityControl.value || !this.cities.includes(cityControl.value))) {
      cityControl?.setErrors({ required: true });
      this.markFormGroupTouched(this.billingAddressForm);
      return;
    }

    if (this.billingAddressForm.valid) {
      this.isSavingBillingAddress = true;

      this.billingPaymentDetailsService.updateBillingAddress(this.billingAddressForm.value).subscribe({
        next: () => {
          this.generalService.showNotification({
            message: 'Billing address updated successfully!',
            style: 'success'
          });
          this.isEditingBillingAddress = false;
          this.loadBillingAddress();
          this.isSavingBillingAddress = false;
        },
        error: (error) => {
          console.error('Failed to update billing address:', error);
          this.generalService.showNotification({
            message: 'Failed to update billing address. Please try again.',
            style: 'error'
          });
          this.isSavingBillingAddress = false;
        }
      });
    } else {
      this.markFormGroupTouched(this.billingAddressForm);
    }
  }

  onUpdateVatInfo() {
    if (this.vatForm.valid) {
      this.billingPaymentDetailsService.updateVatInfo(this.vatForm.value).subscribe({
        next: () => {
          this.generalService.showNotification({
            message: 'VAT information updated successfully!',
            style: 'success'
          });
          this.isEditingVat = false;
          this.loadVatInfo(); // Refresh the data
        },
        error: (error) => {
          console.error('Failed to update VAT information:', error);
          this.generalService.showNotification({
            message: 'Failed to update VAT information. Please try again.',
            style: 'error'
          });
        }
      });
    } else {
      this.markFormGroupTouched(this.vatForm);
    }
  }

  getCountryName(countryCode: string): string {
    return this.billingLocationService.getCountryName(countryCode, this.vatCountries, this.countries);
  }

  hasBillingAddress(): boolean {
    if (!this.billingAddressDetails) {
      return false;
    }
    // Check if at least one address field has a value
    return !!(
      this.billingAddressDetails.addressLine1 ||
      this.billingAddressDetails.city ||
      this.billingAddressDetails.state ||
      this.billingAddressDetails.zipCode ||
      this.billingAddressDetails.country
    );
  }

  private updateStateControlValidation() {
    const stateControl = this.billingAddressForm.get('state');
    if (!stateControl) return;

    if (this.states.length > 0) {
      stateControl.setValidators([Validators.required]);
    } else {
      stateControl.clearValidators();
    }
    stateControl.updateValueAndValidity({ emitEvent: false });
  }

  private updateCityControlValidation() {
    const cityControl = this.billingAddressForm.get('city');
    if (!cityControl) return;

    if (this.cities.length > 0) {
      cityControl.setValidators([Validators.required]);
    } else {
      cityControl.clearValidators();
    }
    cityControl.updateValueAndValidity({ emitEvent: false });
  }

  private withPreferredCity(cities: string[], preferredCity: string): string[] {
    return this.billingLocationService.withPreferredCity(cities, preferredCity);
  }

  private findMatchingCity(cities: string[], preferredCity: string): string {
    return this.billingLocationService.findMatchingCity(cities, preferredCity);
  }

  private isCurrentSubscriptionPlan(planIdForApi: string, planName: string): boolean {
    const plan = this.currentSubscription?.plan;
    if (!plan) {
      return false;
    }

    const currentPlanId = plan.id || '';
    const currentPlanName = this.normalizePlanName(plan.name || '');
    const selectedPlanName = this.normalizePlanName(planName || '');

    const sameId = !!planIdForApi && currentPlanId === planIdForApi;
    const sameName =
      !!currentPlanName &&
      !!selectedPlanName &&
      currentPlanName === selectedPlanName;

    return sameId || sameName;
  }

  private normalizePlanName(name: string): string {
    return (name || '')
      .toLowerCase()
      .replace(/\s+/g, ' ')
      .trim();
  }

  hasVatInfo(): boolean {
    if (!this.vatInfoDetails) {
      return false;
    }
    // Check if VAT number and country are set (businessName might be from org name)
    return !!(this.vatInfoDetails.vatNumber && this.vatInfoDetails.country);
  }

  hasPaymentMethod(): boolean {
    if (!this.paymentMethodDetails) {
      return false;
    }
    // Check if last4 is set
    return !!this.paymentMethodDetails.last4;
  }


  private markFormGroupTouched(formGroup: FormGroup) {
    Object.keys(formGroup.controls).forEach(key => {
      const control = formGroup.get(key);
      control?.markAsTouched();
    });
  }


  getCardIconSvg() {
    return this.cardIconService.getCardIconSvg(this.paymentMethodDetails?.brand);
  }
}