import {ChangeDetectorRef, Component, ElementRef, HostListener, OnInit, ViewChild} from '@angular/core';
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
@Component({
  selector: 'app-billing-page',
  templateUrl: './billing-page.component.html',
  styleUrls: ['./billing-page.component.scss']
})
export class BillingPageComponent implements OnInit {
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
  cities: string[] = [];
  isLoadingCountries = false;
  isLoadingVatCountries = false;
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

  ngOnInit() {
    this.validateOrganisation();
    this.loadCountries(); // Load immediately, independent of bootstrap
    this.loadBillingConfiguration();

    // Start bootstrap in background - code that needs it will await the promise
    this.bootstrapSubscriptionPromise = this.bootstrapOrganisation();
    this.overviewService.setBootstrapPromise(this.bootstrapSubscriptionPromise);

    this.billingAddressForm.get('country')?.valueChanges.subscribe(countryCode => {
      this.onCountryChange(countryCode);
    });

    this.vatForm.get('country')?.valueChanges.subscribe(countryCode => {
      this.onVatCountryChange(countryCode);
    });

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
    this.isLoadingBillingData = true;
    try {
      const orgId = this.getOrganisationId();
      const [usageResponse, paymentResponse] = await Promise.all([
        this.httpService.request({
          url: `/billing/organisations/${orgId}/usage`,
          method: 'get',
          hideNotification: true
        }).catch(() => ({ data: null })),
        this.httpService.request({
          url: `/billing/organisations/${orgId}/payment_methods`,
          method: 'get',
          hideNotification: true
        }).catch(() => ({ data: null }))
      ]);

      const subscriptionResponse = await this.httpService.request({
        url: `/billing/organisations/${orgId}/subscription`,
        method: 'get',
        hideNotification: true
      }).catch(() => ({ data: null }));

      const hadSubscription = this.hasActiveSubscription(this.currentSubscription);
      const hasSubscription = this.hasActiveSubscription(subscriptionResponse.data);
      if (hadSubscription !== hasSubscription) {
        this.licensesService.setLicenses().catch(() => {});
      }

      const overviewData = {
        subscription: subscriptionResponse.data,
        usage: usageResponse.data,
        payment: paymentResponse.data
      };

      this.currentSubscription = subscriptionResponse.data;

      if (overviewData) {
        this.billingOverview = this.overviewService.formatOverviewData(overviewData);

        if (overviewData.usage) {
          this.usageRows = this.usageService.formatUsageData(overviewData.usage);
        } else {
          this.usageRows = [];
        }

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
      await this.loadOrganisationData();
    } catch (error) {
      console.error('Failed to load billing data:', error);
      this.isLoadingBillingData = false;
    }
  }

  private async loadOrganisationData() {
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

  private loadBillingConfiguration() {
    this.billingPaymentDetailsService.getBillingConfig().subscribe({
      next: (config) => {
        this.paymentProviderType = config.data.payment_provider.type;
        this.paymentProviderPublishableKey = config.data.payment_provider.publishable_key;
        this.loadInternalOrganisationId();
      },
      error: (error) => {
        console.error('Failed to load billing configuration:', error);
        this.generalService.showNotification({
          message: 'Failed to load billing configuration. Please refresh the page.',
          style: 'error'
        });
      }
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
        const errorMessage = error?.error?.message || error?.message || 'Failed to load organisation data';
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

  onCountryChange(countryCode: string) {
    if (!countryCode) {
      this.cities = [];
      this.billingAddressForm.get('city')?.setValue('');
      return;
    }

    this.billingAddressForm.get('city')?.setValue('');
    this.cities = [];

    const countryName = this.getCountryName(countryCode);

    this.isLoadingCities = true;
    this.countriesService.getCitiesForCountry(countryName).subscribe({
      next: (cities) => {
        this.cities = cities;
        this.isLoadingCities = false;
      },
      error: (error) => {
        console.error('Failed to load cities:', error);
        this.isLoadingCities = false;
        this.cities = [];
      }
    });
  }

  private initializeForms() {
    this.billingAddressForm = this.fb.group({
      name: ['', [Validators.required, Validators.minLength(2), Validators.maxLength(100)]],
      addressLine1: ['', [Validators.required, Validators.minLength(5), Validators.maxLength(200)]],
      addressLine2: ['', [Validators.maxLength(200)]],
      country: ['', Validators.required],
      city: ['', Validators.required],
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

  private loadPlans() {
    this.isLoadingPlans = true;

    this.planService.getPlans().subscribe({
      next: (response) => {
        const defaultData = this.planService.getDefaultPlanComparison();

        if (!response.data || response.data.length === 0) {
          this.plans = defaultData.plans;
          this.overwatchPlans = [];
        } else {
          this.overwatchPlans = response.data;

          const overwatchPlansMap = new Map<string, Plan>();
          response.data.forEach((plan: Plan) => {
            overwatchPlansMap.set(plan.name.toLowerCase(), plan);
          });

          this.plans = defaultData.plans.map((defaultPlan: Plan) => {
            const overwatchPlan = overwatchPlansMap.get(defaultPlan.name.toLowerCase());

            if (overwatchPlan) {
              if (!overwatchPlan.features || overwatchPlan.features.length === 0) {
                return {
                  ...overwatchPlan,
                  features: defaultPlan.features,
                  description: overwatchPlan.description || defaultPlan.description,
                  price: overwatchPlan.price || defaultPlan.price,
                  currency: overwatchPlan.currency || defaultPlan.currency,
                  interval: overwatchPlan.interval || defaultPlan.interval
                };
              }
              return overwatchPlan;
            }

            return defaultPlan;
          });
        }
        this.isLoadingPlans = false;
      },
      error: (error) => {
        console.warn('Failed to load plans from backend:', error);
        const defaultData = this.planService.getDefaultPlanComparison();
        this.plans = defaultData.plans;
        this.overwatchPlans = [];
        this.isLoadingPlans = false;
      }
    });
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

    const planName = selectedPlanData.name.toLowerCase();
    const isProOrEnterprise = planName.includes('pro') || planName.includes('enterprise');

    const planLower = planName.toLowerCase();
    const planExistsInOverwatch = this.overwatchPlans.some(p => {
          const pNameLower = p.name.toLowerCase();
          return (planLower.includes(pNameLower) || pNameLower.includes(planLower)) || p.id === selectedPlanData.id;
      });

    // Billing service (Overwatch) expects plan UUID, not slug; resolve from overwatch when selected plan is default slug
    const overwatchPlan = this.overwatchPlans.find(p => {
      const pNameLower = p.name.toLowerCase();
      return (planLower.includes(pNameLower) || pNameLower.includes(planLower)) || p.id === selectedPlanData.id;
    });
    const planIdForApi = overwatchPlan?.id ?? selectedPlanData.id;

    if (isProOrEnterprise && !planExistsInOverwatch) {
      const subject = encodeURIComponent(`${selectedPlanData.name} Plan Request`);
      const body = encodeURIComponent(`Hello,\n\nI would like to subscribe to the ${selectedPlanData.name} plan.\n\nThank you.`);
      window.location.href = `mailto:support@getconvoy.io?subject=${subject}&body=${body}`;
      return;
    }

    this.isLoadingCheckout = true;
    try {
      const orgId = this.getOrganisationId();
      const host = window.location.origin;

      let checkoutUrl: string;

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
    // Compare by plan ID or plan name (case-insensitive)
    const currentPlanId = this.currentSubscription.plan.id;
    const currentPlanName = this.currentSubscription.plan.name?.toLowerCase();
    const plan = this.plans.find(p => p.id === planId);

    if (!plan) return false;

    return plan.id === currentPlanId || plan.name.toLowerCase() === currentPlanName;
  }

  getButtonText(planId: string): string {
    if (this.isLoadingCheckout && this.selectedPlan === planId) {
      return 'Loading...';
    }
    if (this.isCurrentPlan(planId)) {
      return 'Current Plan';
    }
    if (this.selectedPlan === planId) {
      return this.currentSubscription ? 'Upgrade' : 'Subscribe';
    }
    return 'Select';
  }

  getCancelButtonText(): string {
    return this.isCancellingSubscription ? 'Cancelling...' : 'Cancel Subscription';
  }

  selectPlan(planId: string) {
    this.selectedPlan = planId;
  }

  getFeaturesByCategory(category: 'core' | 'security' | 'support'): any[] {
    if (this.plans.length === 0) return [];

    const allFeatures = this.plans.flatMap(plan =>
      plan.features.filter(feature => feature.category === category)
    );

    const uniqueFeatures = allFeatures.filter((feature, index, self) =>
      index === self.findIndex(f => f.name === feature.name)
    );

    return uniqueFeatures;
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
        error: (error) => {
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
      this.billingAddressForm.patchValue(formData);
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
      error: (error) => {
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

    // Wait for webhook to process before loading payment method details
    // Stripe sends a webhook to billing service which processes asynchronously
    setTimeout(() => {
      this.loadPaymentMethodDetailsWithRetry();
      this.loadPaymentMethods();
    }, 1500); // Initial delay to allow webhook processing
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
    const cityControl = this.billingAddressForm.get('city');
    if (!cityControl || !cityControl.value || !this.cities.includes(cityControl.value)) {
      cityControl?.setErrors({ required: true });
      this.markFormGroupTouched(this.billingAddressForm);
      return;
    }

    if (this.billingAddressForm.valid) {
      this.isSavingBillingAddress = true;

      this.billingPaymentDetailsService.updateBillingAddress(this.billingAddressForm.value).subscribe({
        next: (response) => {
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
        next: (response) => {
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
      this.billingAddressDetails.zipCode ||
      this.billingAddressDetails.country
    );
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

      for (const [country, pattern] of Object.entries(vatPatterns)) {
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

      for (const [country, pattern] of Object.entries(zipPatterns)) {
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
