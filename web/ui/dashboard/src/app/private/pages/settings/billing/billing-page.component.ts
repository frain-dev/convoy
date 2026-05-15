import {ChangeDetectorRef, Component, ElementRef, HostListener, OnDestroy, OnInit, ViewChild} from '@angular/core';
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
import {Plan, PlanService} from './plan.service';
import {BillingOverviewService, BillingOverview} from './billing-overview.service';
import {BillingUsageService, UsageRow} from './billing-usage.service';
import {HttpService} from 'src/app/services/http/http.service';
import {LicensesService} from 'src/app/services/licenses/licenses.service';
import {buildCheckoutPayload} from './plan-cadence.util';
import {subscriptionPlanKey, writeCheckoutPlanBaseline} from './checkout-plan-baseline.util';
import {Subscription} from 'rxjs';
import {
  BillingPlansUnavailableReason,
  areOverwatchPlansAvailable,
  BILLING_PLANS_UNAVAILABLE_MESSAGE,
  mapOverwatchPlansForCheckout,
  resolveBillingPlansUnavailableMessage,
  scopePlansForBillingMode,
  shouldFetchPlans
} from './billing-plans.util';
@Component({
    selector: 'app-billing-page',
    templateUrl: './billing-page.component.html',
    styleUrls: ['./billing-page.component.scss'],
    standalone: false
})
export class BillingPageComponent implements OnInit, OnDestroy {
  @ViewChild('paymentDetailsDialog') paymentDetailsDialog!: ElementRef<HTMLDialogElement>;
  @ViewChild('managePlanDialog') managePlanDialog!: ElementRef<HTMLDialogElement>;
  @ViewChild('cancelConfirmDialog') cancelConfirmDialog!: ElementRef<HTMLDialogElement>;

  isPaymentDetailsOpen = false;
  isManagePlanOpen = false;
  isCancelConfirmOpen = false;
  refreshOverviewTrigger = 0;
  selectedPlan: string | null = null;
  currentYear = new Date().getFullYear() - 2000; // 2-digit current year
  currentMonth = new Date().getMonth() + 1; // Current month (1-12)

  plans: Plan[] = [];
  isLoadingPlans = false;
  currentSubscription: any = null;
  overwatchPlans: Plan[] = [];
  plansUnavailableMessage = '';
  hasLoadedPlans = false;
  hasAttemptedPlansLoad = false;
  plansUnavailableReason: BillingPlansUnavailableReason = 'none';

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

  billingAddressForm!: FormGroup;
  vatForm!: FormGroup;

  countries: { code: string; name: string }[] = [];
  vatCountries: { code: string; name: string }[] = []; // Countries with tax ID types from billing service
  taxIdTypes: any[] = []; // Store tax ID types with examples
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

  selfHostedBilling = false;
  selfHostedBootstrapEmail = '';
  selfHostedVerifyCode = '';
  selfHostedBootstrapBusy = false;
  selfHostedBootstrapMessage = '';
  selfHostedLicenseMasked = '';
  selfHostedLicenseReady = true;
  selfHostedHasEntitlements = true;

  constructor(
    private fb: FormBuilder,
    private billingPaymentDetailsService: BillingPaymentDetailsService,
    private generalService: GeneralService,
    private cardIconService: CardIconService,
    private countriesService: CountriesService,
    private planService: PlanService,
    private cdr: ChangeDetectorRef,
    private overviewService: BillingOverviewService,
    private usageService: BillingUsageService,
    private httpService: HttpService,
    private licensesService: LicensesService,
    private route: ActivatedRoute,
    private router: Router
  ) {
    this.initializeForms();
  }

  private bootstrapSubscriptionPromise: Promise<void> | null = null;
  private checkoutVerifiedSub?: Subscription;
  private skipNextBootstrapSubscriptionProbe = false;
  private locationRequestToken = 0;
  private activeCountryRequestToken = 0;
  private activeCityRequestToken = 0;
  private cityLoadingRequestToken: number | null = null;

  async ngOnInit() {
    this.validateOrganisation();
    this.loadCountries(); // Load immediately, independent of bootstrap
    await this.loadBillingConfiguration();
    this.applySelfHostedBootstrapPathFromConfig();
    if (this.selfHostedBilling && !this.selfHostedHasEntitlements && this.canShowBillingPanels) {
      await this.loadBillingData();
    }

    this.checkoutVerifiedSub = this.billingPaymentDetailsService.onCheckoutSubscriptionVerified.subscribe(() => {
      void this.reloadBillingAfterCheckoutPoll();
    });

    this.billingAddressForm.get('country')?.valueChanges.subscribe(countryCode => {
      this.onCountryChange(countryCode);
    });
    this.billingAddressForm.get('state')?.valueChanges.subscribe(stateName => {
      this.onStateChange(stateName);
    });

    this.vatForm.get('country')?.valueChanges.subscribe(countryCode => {
      this.onVatCountryChange(countryCode);
    });

  }

  ngOnDestroy(): void {
    this.checkoutVerifiedSub?.unsubscribe();
  }

  private async reloadBillingAfterCheckoutPoll(): Promise<void> {
    if (this.bootstrapSubscriptionPromise) {
      await this.bootstrapSubscriptionPromise.catch(() => {});
    }
    await this.loadBillingConfiguration();
    this.skipNextBootstrapSubscriptionProbe = true;
    this.applySelfHostedBootstrapPathFromConfig();
    if (this.bootstrapSubscriptionPromise) {
      await this.bootstrapSubscriptionPromise.catch(() => {});
    }
    if (this.selfHostedBilling && !this.selfHostedHasEntitlements && this.canShowBillingPanels) {
      await this.loadBillingData();
    }
    this.cdr.detectChanges();
    this.billingPaymentDetailsService.scheduleInvoiceListCatchupAfterWebhookDelay();
  }

  private applySelfHostedBootstrapPathFromConfig(): void {
    if (this.shouldShowSelfHostedSetup()) {
      this.bootstrapSubscriptionPromise = null;
      this.overviewService.setBootstrapPromise(null);
      this.markBillingDataIdle();
      this.selfHostedBootstrapMessage = this.selfHostedLicenseMasked
        ? 'This configured license is not recognized by billing. Verify your billing email or update the server license key.'
        : 'Verify your billing email to issue a license for this organisation.';
      return;
    }
    if (this.selfHostedBilling && !this.selfHostedHasEntitlements) {
      this.bootstrapSubscriptionPromise = null;
      this.overviewService.setBootstrapPromise(null);
      this.markBillingDataIdle();
      this.selfHostedBootstrapMessage = '';
      return;
    }
    this.bootstrapSubscriptionPromise = this.bootstrapOrganisation();
    this.overviewService.setBootstrapPromise(this.bootstrapSubscriptionPromise);
  }

  private async bootstrapOrganisation() {
    if (this.skipNextBootstrapSubscriptionProbe) {
      this.skipNextBootstrapSubscriptionProbe = false;
      await this.loadBillingData();
      return;
    }

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
      if (this.selfHostedBilling && this.isInvalidLicenseError(error)) {
        this.selfHostedLicenseReady = false;
        this.selfHostedHasEntitlements = false;
        this.selfHostedBootstrapMessage = 'This organisation does not have a valid self-hosted billing license yet.';
        this.markBillingDataIdle();
        return;
      }
      await this.loadBillingData();
    }
  }

  private async loadBillingData() {
    this.isLoadingBillingData = true;
    this.isLoadingUsage = true;
    try {
      const orgId = this.getOrganisationId();
      const paymentResponse = await this.httpService
        .request({
          url: `/billing/organisations/${orgId}/payment_methods`,
          method: 'get',
          hideNotification: true
        })
        .catch(() => ({ data: null }));

      const subscriptionResponse = await this.httpService
        .request({
          url: `/billing/organisations/${orgId}/subscription`,
          method: 'get',
          hideNotification: true
        })
        .catch(() => ({ data: null }));

      const hadSubscription = this.hasActiveSubscription(this.currentSubscription);
      const hasSubscription = this.hasActiveSubscription(subscriptionResponse.data);
      if (hadSubscription !== hasSubscription) {
        this.licensesService.setLicenses().catch(() => {});
      }

      const overviewData = {
        subscription: subscriptionResponse.data,
        usage: null as any,
        payment: paymentResponse.data
      };

      this.currentSubscription = subscriptionResponse.data;

      if (overviewData) {
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
      } else {
        this.billingOverview = null;
        this.usageRows = [];
        this.paymentMethods = [];
      }

      this.isLoadingBillingData = false;
      this.loadUsageSeparately();
      await this.loadOrganisationData();
      this.cdr.detectChanges();
    } catch (error) {
      console.error('Failed to load billing data:', error);
      this.isLoadingBillingData = false;
      this.isLoadingUsage = false;
    }
  }

  private loadUsageSeparately() {
    const orgId = this.getOrganisationId();
    this.httpService
      .request({
        url: `/billing/organisations/${orgId}/usage`,
        method: 'get',
        hideNotification: true
      })
      .then(res => {
        if (res?.data) {
          this.usageRows = this.usageService.formatUsageData(res.data);
        } else {
          this.usageRows = [];
        }
      })
      .catch(() => {
        this.usageRows = [];
      })
      .finally(() => {
        this.isLoadingUsage = false;
        this.cdr.detectChanges();
      });
  }

  private async loadOrganisationData() {
    // loadBillingData (only caller) runs inside bootstrapOrganisation while
    // bootstrapSubscriptionPromise is pending; awaiting it here deadlocks.

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
    try {
      const org = localStorage.getItem('CONVOY_ORG');

      if (!org) {
        throw new Error('No organisation found in localStorage');
      }

      const orgData = JSON.parse(org);

      if (!orgData.uid) {
        throw new Error('No organisation UID found');
      }
    } catch (error) {
      this.generalService.showNotification({
        message: 'Invalid organisation data. Please refresh the page and try again.',
        style: 'error'
      });
    }
  }

  private loadBillingConfiguration(): Promise<void> {
    return new Promise(resolve => {
      this.billingPaymentDetailsService.getBillingConfig(this.getOrganisationId()).subscribe({
      next: (config) => {
        this.selfHostedBilling = !!config.data?.self_hosted;
        const licenseSummary = config.data?.license;
        this.selfHostedLicenseReady = !this.selfHostedBilling || !!licenseSummary?.configured;
        this.selfHostedHasEntitlements = !this.selfHostedBilling || !!licenseSummary?.has_entitlements;
        this.selfHostedLicenseMasked = licenseSummary?.masked_key || '';
        this.paymentProviderType = config.data?.payment_provider?.type || '';
        this.paymentProviderPublishableKey = config.data?.payment_provider?.publishable_key || '';
        if (this.canShowBillingPanels) {
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
        resolve();
      }
    });
    });
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
        if (this.selfHostedBilling && this.isInvalidLicenseError(error)) {
          this.selfHostedLicenseReady = false;
          this.selfHostedHasEntitlements = false;
          this.selfHostedBootstrapMessage = 'This organisation does not have a valid self-hosted billing license yet.';
          this.markBillingDataIdle();
          return;
        }
        const errorMessage = this.billingErrorMessage(error, 'Failed to load organisation data');
        this.generalService.showNotification({
          message: errorMessage,
          style: 'error'
        });
        this.internalOrganisationId = '';
        // Don't load existing data if the first call failed
      }
    });
  }

  get canShowBillingPanels(): boolean {
    return !this.selfHostedBilling || this.selfHostedLicenseReady;
  }

  get showSelfHostedBillingCard(): boolean {
    return this.selfHostedBilling && (!this.selfHostedLicenseReady || !this.selfHostedHasEntitlements);
  }

  private shouldShowSelfHostedSetup(): boolean {
    return this.selfHostedBilling && !this.selfHostedLicenseReady;
  }

  private isInvalidLicenseError(error: any): boolean {
    const message = this.billingErrorMessage(error, '').toLowerCase();
    return message.includes('invalid license') || message.includes('no organisation license configured');
  }

  private billingErrorMessage(error: any, fallback: string): string {
    if (typeof error === 'string') return error;
    return error?.error?.message || error?.response?.data?.message || error?.message || fallback;
  }

  private markBillingDataIdle(): void {
    this.billingOverview = null;
    this.usageRows = [];
    this.paymentMethods = [];
    this.paymentMethodDetails = null;
    this.internalOrganisationId = '';
    this.isLoadingBillingData = false;
    this.isLoadingUsage = false;
    this.isLoadingBillingAddress = false;
    this.isLoadingVat = false;
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
    this.billingAddressForm = this.fb.group({
      name: ['', [Validators.required, Validators.minLength(2), Validators.maxLength(100)]],
      addressLine1: ['', [Validators.required, Validators.minLength(5), Validators.maxLength(200)]],
      addressLine2: ['', [Validators.maxLength(200)]],
      country: ['', Validators.required],
      state: [''],
      city: [''],
      zipCode: ['', [Validators.required, Validators.minLength(3), Validators.maxLength(20), this.zipCodeValidator()]]
    });

    this.vatForm = this.fb.group({
      businessName: ['', [Validators.required, Validators.minLength(2), Validators.maxLength(200)]],
      country: ['', Validators.required],
      vatNumber: ['', [Validators.required, this.vatNumberValidator()]]
    });
  }

  openPaymentDetails() {
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

  retryPlansLoad() {
    this.loadPlans(true);
  }

  private loadPlans(forceReload = false) {
    if (!shouldFetchPlans(this.hasLoadedPlans, this.isLoadingPlans, forceReload)) {
      this.hasAttemptedPlansLoad = true;
      return;
    }

    this.hasAttemptedPlansLoad = true;
    this.isLoadingPlans = true;
    this.plansUnavailableMessage = '';
    this.plansUnavailableReason = 'none';

    this.planService.getPlans(this.getOrganisationId()).subscribe({
      next: (response) => {
        const plansFromApi = Array.isArray(response.data) ? response.data : [];
        if (plansFromApi.length === 0) {
          this.plans = [];
          this.overwatchPlans = [];
          this.selectedPlan = null;
          this.hasLoadedPlans = false;
          this.plansUnavailableReason = 'empty_catalog';
          this.plansUnavailableMessage = resolveBillingPlansUnavailableMessage(this.plansUnavailableReason, this.selfHostedBilling);
        } else {
          const mappedPlans = mapOverwatchPlansForCheckout(plansFromApi);
          const scopedPlans = scopePlansForBillingMode(mappedPlans, this.selfHostedBilling);
          this.overwatchPlans = scopedPlans;
          this.plans = scopedPlans;
          this.hasLoadedPlans = scopedPlans.length > 0;

          if (this.selectedPlan && !this.plans.some(plan => plan.id === this.selectedPlan)) {
            this.selectedPlan = null;
          }

          if (!this.hasLoadedPlans) {
            this.plansUnavailableReason = 'mode_filtered';
            this.plansUnavailableMessage = resolveBillingPlansUnavailableMessage(this.plansUnavailableReason, this.selfHostedBilling);
          }
        }
        this.isLoadingPlans = false;
      },
      error: (error) => {
        console.warn('Failed to load plans from backend:', error);
        this.plans = [];
        this.overwatchPlans = [];
        this.selectedPlan = null;
        this.hasLoadedPlans = false;
        this.plansUnavailableReason = 'fetch_error';
        this.plansUnavailableMessage = resolveBillingPlansUnavailableMessage(this.plansUnavailableReason, this.selfHostedBilling);
        this.isLoadingPlans = false;
      }
    });
  }

  get showCloudManagedPlansState(): boolean {
    return this.plansUnavailableReason === 'mode_filtered' && !this.selfHostedBilling;
  }

  openCloudWorkspaceBilling(): void {
    // Route user to Convoy Cloud billing settings explicitly instead of a blank retry loop.
    window.open('https://app.getconvoy.io/settings?activePage=usage%20and%20billing', '_blank', 'noopener,noreferrer');
  }

  closeManagePlan() {
    this.isManagePlanOpen = false;
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
      const subscriptionId = this.currentSubscription.id;
      await this.httpService.request({
        url: `/billing/organisations/${orgId}/subscriptions/${subscriptionId}`,
        method: 'delete'
      });

      this.closeManagePlan();

      this.generalService.showNotification({
        message: 'Subscription cancelled successfully',
        style: 'success'
      });

      await this.loadBillingConfiguration();
      this.bootstrapSubscriptionPromise = null;
      this.overviewService.setBootstrapPromise(null);
      this.applySelfHostedBootstrapPathFromConfig();
      await this.loadBillingData();
      this.refreshOverviewTrigger++;
      void this.licensesService.setLicenses();
      this.cdr.detectChanges();
    } catch (error: any) {
      console.error('Failed to cancel subscription:', error);
      this.generalService.showNotification({
        message: this.billingErrorMessage(error, 'Failed to cancel subscription. Please try again.'),
        style: 'error'
      });
    } finally {
      this.isCancellingSubscription = false;
    }
  }

  async onUpgradePlan() {
    if (!this.hasPlanReadyState || !this.areCheckoutPlansAvailable()) {
      this.generalService.showNotification({
        message: this.plansUnavailableMessage || BILLING_PLANS_UNAVAILABLE_MESSAGE,
        style: 'error'
      });
      return;
    }

    if (!this.canCheckoutSelectedPlan()) {
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

    const planName = selectedPlanData.name.toLowerCase();
    const isProOrEnterprise = planName.includes('pro') || planName.includes('enterprise');

    const { planExistsInOverwatch, planIdForApi } = this.resolvePlanForApi(selectedPlanData);

    if (this.isCurrentSubscriptionPlan(planIdForApi, selectedPlanData.name)) {
      this.generalService.showNotification({
        message: 'You are already on this plan',
        style: 'success'
      });
      return;
    }

    if (isProOrEnterprise && !planExistsInOverwatch && !this.selfHostedBilling) {
      const subject = encodeURIComponent(`${selectedPlanData.name} Plan Request`);
      const body = encodeURIComponent(`Hello,\n\nI would like to subscribe to the ${selectedPlanData.name} plan.\n\nThank you.`);
      window.location.href = `mailto:support@getconvoy.io?subject=${subject}&body=${body}`;
      return;
    }

    if (this.selfHostedBilling && !planExistsInOverwatch) {
      this.generalService.showNotification({
        message: 'This self-hosted plan is not available from billing yet. Refresh plans or contact support.',
        style: 'error'
      });
      return;
    }

    this.isLoadingCheckout = true;
    try {
      const orgId = this.getOrganisationId();
      const host = window.location.origin;
      const payload = buildCheckoutPayload(planIdForApi, host, selectedPlanData);

      let checkoutUrl: string;

      if (this.currentSubscription && this.currentSubscription.id) {
        const response = await this.httpService.request({
          url: `/billing/organisations/${orgId}/subscriptions/${this.currentSubscription.id}/upgrade`,
          method: 'put',
          body: payload
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
          body: payload
        });

        if (response.data && response.data.checkout_url) {
          checkoutUrl = response.data.checkout_url;
        } else {
          throw new Error('Checkout URL not found in response');
        }
      }

      writeCheckoutPlanBaseline(orgId, subscriptionPlanKey(this.currentSubscription));

      // Open in same window since callback will redirect back
      window.location.href = checkoutUrl;
    } catch (error: any) {
      this.isLoadingCheckout = false;
      console.error('Failed to create checkout session:', error);
      this.generalService.showNotification({
        message: this.billingErrorMessage(error, 'Failed to create checkout session. Please try again.'),
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

    const { planIdForApi } = this.resolvePlanForApi(plan);
    return this.isCurrentSubscriptionPlan(planIdForApi, plan.name);
  }

  getButtonText(planId: string): string {
    if (!this.areCheckoutPlansAvailable()) {
      return 'Unavailable';
    }

    if (this.isLoadingCheckout && this.selectedPlan === planId) {
      return 'Loading...';
    }
    if (this.isCurrentPlan(planId)) {
      return 'Current Plan';
    }
    if (this.selectedPlan === planId) {
      if (!this.hasActiveSubscription(this.currentSubscription)) {
        return 'Subscribe';
      }
      return this.planSwitchButtonLabel(planId);
    }
    return 'Select';
  }

  /** CTA when changing an existing subscription (selected card). */
  private planSwitchButtonLabel(planId: string): string {
    const target = this.plans.find(p => p.id === planId);
    if (!target) {
      return 'Switch plan';
    }

    const currentCatalog = this.plans.find(p => this.isCurrentPlan(p.id)) ?? null;
    const targetPricing = this.resolvePlanPricing(target);

    let currentPricing = currentCatalog ? this.resolvePlanPricing(currentCatalog) : null;
    if (!currentPricing && this.currentSubscription?.plan) {
      currentPricing = this.resolvePlanPricing(this.currentSubscription.plan as Plan);
    }

    if (targetPricing && currentPricing) {
      const diff = this.toMonthlyAmount(targetPricing) - this.toMonthlyAmount(currentPricing);
      const eps = 0.005;
      if (diff > eps) return 'Upgrade';
      if (diff < -eps) return 'Downgrade';
      return 'Switch plan';
    }

    if (currentCatalog) {
      const ti = this.plans.indexOf(target);
      const ci = this.plans.indexOf(currentCatalog);
      if (ti >= 0 && ci >= 0 && ti !== ci) {
        return ti > ci ? 'Upgrade' : 'Downgrade';
      }
    }

    return 'Switch plan';
  }

  getPlanPricingDisplay(plan: Plan): { from: string; amount: string; cadence: string; helperText: string | null } {
    const pricing = this.resolvePlanPricing(plan);
    if (!pricing) {
      return {
        from: '',
        amount: 'Custom pricing',
        cadence: '',
        helperText: 'Final amount shown at checkout'
      };
    }

    const normalizedCadence = this.getHighestTierCadence();
    if (normalizedCadence === 'month' && pricing.interval === 'year') {
      return {
        from: 'from',
        amount: this.formatCurrencyAmount(pricing.amount / 12, pricing.currency),
        cadence: '/ month',
        helperText: 'Billed annually'
      };
    }

    if (normalizedCadence === 'year' && pricing.interval === 'month') {
      return {
        from: 'from',
        amount: this.formatCurrencyAmount(pricing.amount * 12, pricing.currency),
        cadence: '/ year',
        helperText: 'Billed monthly'
      };
    }

    return {
      from: 'from',
      amount: this.formatCurrencyAmount(pricing.amount, pricing.currency),
      cadence: `/ ${pricing.interval}`,
      helperText: null
    };
  }

  private getHighestTierCadence(): 'month' | 'year' | null {
    const pricedPlans = this.plans
      .map(plan => this.resolvePlanPricing(plan))
      .filter((pricing): pricing is { amount: number; currency: string; interval: string } => pricing !== null)
      .filter(pricing => pricing.interval === 'month' || pricing.interval === 'year');

    if (pricedPlans.length === 0) {
      return null;
    }

    const highestTier = pricedPlans.reduce((highest, current) =>
      this.toMonthlyAmount(current) > this.toMonthlyAmount(highest) ? current : highest
    );

    return highestTier.interval === 'month' ? 'month' : 'year';
  }

  private toMonthlyAmount(pricing: { amount: number; interval: string }): number {
    if (pricing.interval === 'year') {
      return pricing.amount / 12;
    }

    return pricing.amount;
  }

  private resolvePlanPricing(plan: Plan): { amount: number; currency: string; interval: string } | null {
    const pricingOptions = Array.isArray(plan.pricing_options)
      ? plan.pricing_options.filter(option => typeof option?.amount_cents === 'number' && (option.amount_cents as number) > 0)
      : [];

    if (pricingOptions.length > 0) {
      const minOption = pricingOptions.reduce((min, option) =>
        (option.amount_cents as number) < (min.amount_cents as number) ? option : min
      );
      return {
        amount: (minOption.amount_cents as number) / 100,
        currency: (minOption.currency || plan.currency || 'USD').toUpperCase(),
        interval: this.formatPricingInterval(minOption.interval || plan.interval)
      };
    }

    if (typeof plan.price === 'number' && plan.price > 0) {
      return {
        amount: plan.price,
        currency: (plan.currency || 'USD').toUpperCase(),
        interval: this.formatPricingInterval(plan.interval)
      };
    }

    return null;
  }

  private formatPricingInterval(interval?: string): string {
    const value = (interval || '').trim().toLowerCase();
    if (!value) return 'month';

    if (value === 'monthly' || value === 'month') return 'month';
    if (value === 'annual' || value === 'year' || value === 'yearly') return 'year';

    return value;
  }

  private formatCurrencyAmount(amount: number, currency: string): string {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency,
      maximumFractionDigits: amount % 1 === 0 ? 0 : 2
    }).format(amount);
  }

  getCancelButtonText(): string {
    return this.isCancellingSubscription ? 'Cancelling...' : 'Cancel Subscription';
  }

  selectPlan(planId: string) {
    if (!this.areCheckoutPlansAvailable()) {
      return;
    }

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

  /** Handle plan card button: Select selects the plan; Upgrade/Subscribe triggers checkout. */
  onPlanCardButtonClick(planId: string): void {
    if (!this.areCheckoutPlansAvailable()) {
      return;
    }

    if (this.isCurrentPlan(planId)) return;
    if (this.selectedPlan === planId) {
      this.onUpgradePlan();
    } else {
      this.selectPlan(planId);
    }
  }

  get hasPlanLoadingState(): boolean {
    return this.isLoadingPlans;
  }

  get hasPlanReadyState(): boolean {
    return !this.isLoadingPlans && this.plans.length > 0 && this.areCheckoutPlansAvailable();
  }

  get hasPlanUnavailableState(): boolean {
    return this.hasAttemptedPlansLoad && !this.isLoadingPlans && !this.hasPlanReadyState;
  }

  hasCompareData(): boolean {
    if (!this.hasPlanReadyState) {
      return false;
    }

    return ['core', 'security', 'support'].some(category =>
      this.getFeaturesByCategory(category as 'core' | 'security' | 'support').length > 0
    );
  }

  getFeaturesByCategory(category: 'core' | 'security' | 'support'): any[] {
    if (this.plans.length === 0) return [];

    const allFeatures = this.plans.flatMap(plan =>
      plan.features.filter(feature => feature.category === category)
    );

    const uniqueFeatures = allFeatures.filter((feature, index, self) =>
      index === self.findIndex(f => (f.key || f.name) === (feature.key || feature.name))
    );

    return uniqueFeatures;
  }

  getFeatureValue(planId: string, feature: { key?: string; name: string }): string {
    const cell = this.getFeatureCell(planId, feature);
    return cell.value;
  }

  getFeatureBaselineValue(planId: string, feature: { key?: string; name: string }): string {
    const cell = this.getFeatureCell(planId, feature);
    return cell.baselineValue || '';
  }

  isFeatureOverridden(planId: string, feature: { key?: string; name: string }): boolean {
    return this.getFeatureCell(planId, feature).isOverridden;
  }

  getFeatureValueType(planId: string, feature: { key?: string; name: string }): 'supported' | 'unsupported' | 'plain' {
    const value = this.getFeatureValue(planId, feature);

    if (value === 'Supported') return 'supported';
    if (value === 'Unsupported') return 'unsupported';
    return 'plain';
  }

  private getFeatureCell(
    planId: string,
    feature: { key?: string; name: string }
  ): { value: string; baselineValue: string; isOverridden: boolean } {
    const plan = this.plans.find(p => p.id === planId);
    if (!plan) {
      return { value: 'Unsupported', baselineValue: 'Unsupported', isOverridden: false };
    }

    const baselineFeature = plan.features.find(f => {
      if (feature.key && f.key) {
        return f.key === feature.key;
      }

      return f.name === feature.name;
    });

    const baselineValue = baselineFeature ? baselineFeature.value : 'Unsupported';
    const currentPlanColumn = this.isCurrentSubscriptionPlan(plan.id, plan.name);
    if (!currentPlanColumn || !feature.key) {
      return { value: baselineValue, baselineValue, isOverridden: false };
    }

    const entitlement = this.getComputedEntitlement(feature.key);
    if (!entitlement) {
      return { value: baselineValue, baselineValue, isOverridden: false };
    }

    const effectiveValue = this.normalizeEntitlementValue(entitlement.value);
    const isOverridden = effectiveValue !== baselineValue;
    return {
      value: effectiveValue,
      baselineValue,
      isOverridden
    };
  }

  private getComputedEntitlement(featureKey: string): { value: unknown } | null {
    const entitlements = Array.isArray(this.currentSubscription?.computed_entitlements)
      ? this.currentSubscription.computed_entitlements
      : [];

    const match = entitlements.find((entitlement: any) => entitlement?.key === featureKey);
    return match || null;
  }

  private normalizeEntitlementValue(value: unknown): string {
    if (typeof value === 'boolean') {
      return value ? 'Supported' : 'Unsupported';
    }

    if (typeof value === 'number') {
      if (value === -1) {
        return 'Unlimited';
      }

      return value.toLocaleString('en-US');
    }

    if (value === null || typeof value === 'undefined') {
      return 'Unsupported';
    }

    return String(value);
  }

  private loadExistingData() {
    this.loadPaymentMethods();
    this.loadPaymentMethodDetails();
    this.loadBillingAddress();
    this.loadVatInfo();
  }

  private loadPaymentMethods() {
    this.isLoadingPaymentMethod = true;
    this.billingPaymentDetailsService.getPaymentMethods().subscribe({
      next: (methods) => {
        // Sort by ID to maintain consistent order
        this.paymentMethods = methods.sort((a, b) => a.id.localeCompare(b.id));
        this.isLoadingPaymentMethod = false;
      },
      error: (error) => {
        console.error('Failed to load payment methods:', error);
        this.paymentMethods = [];
        this.isLoadingPaymentMethod = false;
      }
    });
  }

  private loadPaymentMethodDetails() {
    this.isLoadingPaymentMethod = true;
    this.billingPaymentDetailsService.getPaymentMethodDetails().subscribe({
      next: (details) => {
        this.paymentMethodDetails = details;
        this.isLoadingPaymentMethod = false;
      },
      error: (error) => {
        console.error('Failed to load payment method details:', error);
        this.isLoadingPaymentMethod = false;
      }
    });
  }

  private loadPaymentMethodDetailsWithRetry(maxRetries: number = 5, retryDelay: number = 1000) {
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
    this.billingPaymentDetailsService.getBillingAddress().subscribe({
      next: (details) => {
        this.billingAddressDetails = details;
        this.isLoadingBillingAddress = false;
      },
      error: (error) => {
        console.error('Failed to load billing address:', error);
        this.billingAddressDetails = null; // Clear any existing data on error
        this.isLoadingBillingAddress = false;
      }
    });
  }

  private loadVatInfo() {
    this.isLoadingVat = true;
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

  // Edit mode methods
  startEditingPaymentMethod() {
    try {
      const org = localStorage.getItem('CONVOY_ORG');
      if (!org) {
        throw new Error('No organisation found');
      }

      const orgData = JSON.parse(org);
      if (!orgData.uid) {
        throw new Error('Invalid organisation data');
      }
    } catch (error) {
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

    setTimeout(() => {
      this.loadPaymentMethodDetailsWithRetry();
      this.loadPaymentMethods();
    }, 1500);
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

  async registerSelfHostedEmail(): Promise<void> {
    const email = (this.selfHostedBootstrapEmail || '').trim();
    if (!email) {
      this.generalService.showNotification({ message: 'Enter your billing email', style: 'error' });
      return;
    }
    this.selfHostedBootstrapBusy = true;
    this.selfHostedBootstrapMessage = '';
    try {
      const organisation_name = this.getOrganisationNameFromStorage();
      const body: { email: string; organisation_name?: string } = { email };
      if (organisation_name) {
        body.organisation_name = organisation_name;
      }
      await this.httpService.request({
        url: '/billing/self_hosted/register_email',
        method: 'post',
        body
      });
      this.selfHostedBootstrapMessage =
        'Check your email for an 8-character verification code.';
      this.generalService.showNotification({ message: 'Verification email sent', style: 'success' });
    } catch (e: any) {
      this.generalService.showNotification({
        message: this.billingErrorMessage(e, 'Could not start registration'),
        style: 'error'
      });
    } finally {
      this.selfHostedBootstrapBusy = false;
      this.cdr.detectChanges();
    }
  }

  async verifySelfHostedEmail(): Promise<void> {
    const code = (this.selfHostedVerifyCode || '').trim().toUpperCase();
    if (!code) {
      this.generalService.showNotification({ message: 'Paste the verification code', style: 'error' });
      return;
    }
    this.selfHostedBootstrapBusy = true;
    try {
      const res = await this.httpService.request({
        url: '/billing/self_hosted/verify_email',
        method: 'post',
        body: { code }
      });
      const maskedLicense = res?.data?.masked_license_key as string | undefined;
      const instructions =
        res?.data?.instructions ||
        'License issued and emailed. Set the license key as CONVOY_LICENSE_KEY, restart Convoy, then refresh this page.';
      const postVerifyNextSteps =
        `${instructions} We also sent the license key to your billing email, so check your inbox (and spam). ` +
        'If needed, you can run verification again.';
      if (maskedLicense) {
        this.selfHostedLicenseMasked = maskedLicense;
      }
      this.selfHostedVerifyCode = '';
      this.selfHostedLicenseReady = false;
      this.selfHostedHasEntitlements = false;
      this.selfHostedBootstrapMessage = postVerifyNextSteps;
      this.markBillingDataIdle();
      this.generalService.showNotification({ message: postVerifyNextSteps, style: 'success' });
    } catch (e: any) {
      this.generalService.showNotification({
        message: this.billingErrorMessage(e, 'Verification failed'),
        style: 'error'
      });
    } finally {
      this.selfHostedBootstrapBusy = false;
      this.cdr.detectChanges();
    }
  }

  getOrganisationId(): string {
    try {
      const org = localStorage.getItem('CONVOY_ORG');
      if (!org) {
        return '';
      }

      const orgData = JSON.parse(org);
      return orgData.uid || '';
    } catch (error) {
      console.error('Error getting organisation ID:', error);
      return '';
    }
  }

  private getOrganisationNameFromStorage(): string {
    try {
      const raw = localStorage.getItem('CONVOY_ORG');
      if (!raw) {
        return '';
      }
      const orgData = JSON.parse(raw) as { name?: string };
      return (orgData.name || '').trim();
    } catch (error) {
      console.error('Error getting organisation name:', error);
      return '';
    }
  }

  private hasActiveSubscription(data: any): boolean {
    return !!(data && (data.id || (data.plan && (data.plan.id || data.plan.name))));
  }

  async onUpdatePaymentMethodWithProvider(stripeElementsComponent: StripeElementsComponent, event?: Event) {
    if (event) {
      event.preventDefault();
      event.stopPropagation();
    }

    this.isSavingPaymentMethod = true;

    await new Promise(resolve => setTimeout(resolve, 100));

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
    let country = this.vatCountries.find(c => c.code === countryCode);
    if (!country) {
      country = this.countries.find(c => c.code === countryCode);
    }
    return country ? country.name : countryCode;
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
    if (!preferredCity) {
      return cities;
    }

    const match = cities.some(city => city.trim().toLowerCase() === preferredCity.trim().toLowerCase());
    if (match) {
      return cities;
    }

    return [preferredCity, ...cities];
  }

  private findMatchingCity(cities: string[], preferredCity: string): string {
    if (!preferredCity) {
      return '';
    }

    return cities.find(city => city.trim().toLowerCase() === preferredCity.trim().toLowerCase()) || '';
  }

  private resolvePlanForApi(selectedPlanData: Plan): { planExistsInOverwatch: boolean; planIdForApi: string } {
    const planLower = selectedPlanData.name.toLowerCase();
    const overwatchPlan = this.overwatchPlans.find(p => {
      const pNameLower = p.name.toLowerCase();
      return (planLower.includes(pNameLower) || pNameLower.includes(planLower)) || p.id === selectedPlanData.id;
    });

    return {
      planExistsInOverwatch: !!overwatchPlan,
      planIdForApi: overwatchPlan?.id ?? selectedPlanData.id
    };
  }

  private isCurrentSubscriptionPlan(planIdForApi: string, planName: string): boolean {
    if (!this.currentSubscription?.plan) {
      return false;
    }

    const currentPlanId = this.currentSubscription.plan.id || '';
    const currentPlanName = this.normalizePlanName(this.currentSubscription.plan.name || '');
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

  areCheckoutPlansAvailable(): boolean {
    return areOverwatchPlansAvailable(this.overwatchPlans);
  }

  canCheckoutSelectedPlan(): boolean {
    return this.hasPlanReadyState && this.areCheckoutPlansAvailable() && this.isSelectedPlanValid();
  }

  private isSelectedPlanValid(): boolean {
    return !!this.selectedPlan && this.plans.some(plan => plan.id === this.selectedPlan);
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


  private vatNumberValidator() {
    return (control: any) => {
      if (!control.value) {
        return null;
      }

      const vatNumber = control.value.trim().toUpperCase();

      // Basic VAT number validation patterns for common countries
      const vatPatterns: { [key: string]: RegExp } = {
        'GB': /^GB\d{3}\s?\d{4}\s?\d{2}\s?\d{3}$/, // GB123 4567 89 012
        'DE': /^DE\d{9}$/, // DE123456789
        'FR': /^FR[A-Z0-9]{2}\d{9}$/, // FR12345678901
        'IT': /^IT\d{11}$/, // IT12345678901
        'ES': /^ES[A-Z0-9]\d{7}[A-Z0-9]$/, // ES12345678A
        'NL': /^NL\d{9}B\d{2}$/, // NL123456789B12
        'BE': /^BE\d{10}$/, // BE1234567890
        'AT': /^ATU\d{8}$/, // ATU12345678
        'DK': /^DK\d{8}$/, // DK12345678
        'SE': /^SE\d{12}$/, // SE123456789012
        'NO': /^NO\d{9}MVA$/, // NO123456789MVA
        'CA': /^CA\d{9}RT\d{4}$/, // CA123456789RT0001
        'AU': /^\d{11}$/, // 12345678901
        'US': /^\d{2}-\d{7}$/, // 12-3456789
        'DEFAULT': /^[A-Z0-9]{5,20}$/ // Generic pattern for other countries
      };

      for (const [, pattern] of Object.entries(vatPatterns)) {
        if (pattern.test(vatNumber)) {
          return null; // Valid VAT number
        }
      }

      // If no specific pattern matches, use generic validation
      if (vatPatterns.DEFAULT.test(vatNumber)) {
        return null; // Acceptable format
      }

      return { invalidVatNumber: true };
    };
  }

  private zipCodeValidator() {
    return (control: any) => {
      if (!control.value) {
        return null;
      }

      const zipCode = control.value.trim();

      // Basic zip code validation patterns for common countries
      const zipPatterns: { [key: string]: RegExp } = {
        'US': /^\d{5}(-\d{4})?$/, // 12345 or 12345-6789
        'CA': /^[A-Za-z]\d[A-Za-z]\s?\d[A-Za-z]\d$/, // A1A 1A1
        'GB': /^[A-Z]{1,2}\d[A-Z\d]?\s?\d[A-Z]{2}$/, // A1 1AA or AA1A 1AA
        'DE': /^\d{5}$/, // 12345
        'FR': /^\d{5}$/, // 12345
        'IT': /^\d{5}$/, // 12345
        'ES': /^\d{5}$/, // 12345
        'NL': /^\d{4}\s?[A-Z]{2}$/, // 1234 AB
        'BE': /^\d{4}$/, // 1234
        'AT': /^\d{4}$/, // 1234
        'DK': /^\d{4}$/, // 1234
        'SE': /^\d{3}\s?\d{2}$/, // 123 45
        'NO': /^\d{4}$/, // 1234
        'AU': /^\d{4}$/, // 1234
        'DEFAULT': /^[A-Z0-9\s\-]{3,10}$/ // Generic pattern for other countries
      };

      for (const [, pattern] of Object.entries(zipPatterns)) {
        if (pattern.test(zipCode)) {
          return null; // Valid zip code
        }
      }

      // If no specific pattern matches, use generic validation
      if (zipPatterns.DEFAULT.test(zipCode)) {
        return null; // Acceptable format
      }

      return { invalidZipCode: true };
    };
  }

  getCardIconSvg() {
    return this.cardIconService.getCardIconSvg(this.paymentMethodDetails?.brand);
  }
}