import {Component, ElementRef, OnInit, ViewChild} from '@angular/core';
import {StripeElementsComponent} from './stripe-elements.component';
import {FormBuilder, FormGroup, Validators} from '@angular/forms';
import {
    BillingAddressDetails,
    BillingPaymentDetailsService,
    PaymentMethodDetails,
    VatInfoDetails
} from './billing-payment-details.service';
import {CardIconService} from './card-icon.service';
import {GeneralService} from 'src/app/services/general/general.service';
import {CountriesService} from 'src/app/services/countries/countries.service';
import {Plan, PlanService} from './plan.service';

@Component({
  selector: 'app-billing-page',
  templateUrl: './billing-page.component.html',
  styleUrls: ['./billing-page.component.scss']
})
export class BillingPageComponent implements OnInit {
  @ViewChild('paymentDetailsDialog') paymentDetailsDialog!: ElementRef<HTMLDialogElement>;
  @ViewChild('managePlanDialog') managePlanDialog!: ElementRef<HTMLDialogElement>;

  isPaymentDetailsOpen = false;
  isManagePlanOpen = false;
  refreshOverviewTrigger = 0;
  selectedPlan: 'pro' | 'enterprise' = 'pro';
  currentYear = new Date().getFullYear() - 2000; // 2-digit current year
  currentMonth = new Date().getMonth() + 1; // Current month (1-12)

  // Plan data
  plans: Plan[] = [];
  isLoadingPlans = false;

  // Existing data
  paymentMethodDetails: PaymentMethodDetails | null = null;
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

  billingAddressForm!: FormGroup;
  vatForm!: FormGroup;

  countries: { code: string; name: string }[] = [];
  vatCountries: { code: string; name: string }[] = []; // Countries with tax ID types from Overwatch
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
  internalOrganisationId = ''; // Internal ID from Overwatch

  constructor(
    private fb: FormBuilder,
    private billingPaymentDetailsService: BillingPaymentDetailsService,
    private generalService: GeneralService,
    private cardIconService: CardIconService,
    private countriesService: CountriesService,
    private planService: PlanService
  ) {
    this.initializeForms();
  }

  ngOnInit() {
    this.validateOrganisation();
    this.loadBillingConfiguration();
    this.loadCountries();
    this.loadExistingData();

    this.billingAddressForm.get('country')?.valueChanges.subscribe(countryCode => {
      this.onCountryChange(countryCode);
    });
  }

  private validateOrganisation() {
    try {
      const org = localStorage.getItem('CONVOY_ORG');
      console.log('Validating organisation from localStorage:', org);

      if (!org) {
        throw new Error('No organisation found in localStorage');
      }

      const orgData = JSON.parse(org);
      console.log('Organisation data:', orgData);

      if (!orgData.uid) {
        throw new Error('No organisation UID found');
      }

      console.log('Organisation validation passed. UID:', orgData.uid);
    } catch (error) {
      console.error('Organisation validation failed:', error);
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
        console.log('Loaded billing config:', config.data);
        console.log('Payment provider type:', this.paymentProviderType);
        console.log('Publishable key:', this.paymentProviderPublishableKey ? 'Present' : 'Missing');

        // Load internal organisation ID from Overwatch
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
        console.log('Loaded internal organisation ID from billing service:', this.internalOrganisationId);
      },
      error: (error) => {
        console.error('Failed to load internal organisation ID:', error);
        this.generalService.showNotification({
          message: 'Failed to load organisation data. Please refresh the page.',
          style: 'error'
        });
        this.internalOrganisationId = '';
      }
    });
  }

  private loadCountries() {
    this.isLoadingCountries = true;
    this.countriesService.getCountries().subscribe({
      next: (countries) => {
        this.countries = countries;
        this.isLoadingCountries = false;
        console.log('Loaded countries:', countries.length);
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
        console.log('Loaded VAT countries from Overwatch:', this.vatCountries);
      },
      error: (error) => {
        console.error('Failed to load VAT countries:', error);
        this.isLoadingVatCountries = false;
        this.vatCountries = [];
      }
    });
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
        console.log(`Loaded ${cities.length} cities for ${countryName} (${countryCode})`);
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
    this.loadPlans();
    this.managePlanDialog.nativeElement.showModal();
  }

  private loadPlans() {
    this.isLoadingPlans = true;

    // Load plans from backend configuration
    this.planService.getPlans().subscribe({
      next: (response) => {
        // If no plans in config, use default plans
        if (!response.data || response.data.length === 0) {
          const defaultData = this.planService.getDefaultPlanComparison();
          this.plans = defaultData.plans;
        } else {
          this.plans = response.data;
        }
        this.isLoadingPlans = false;
      },
      error: (error) => {
        console.warn('Failed to load plans from backend:', error);
        // Use default data when backend fails
        const defaultData = this.planService.getDefaultPlanComparison();
        this.plans = defaultData.plans;
        this.isLoadingPlans = false;
      }
    });
  }

  closeManagePlan() {
    this.managePlanDialog.nativeElement.close();
  }

  onCancelPlan() {
    const subject = encodeURIComponent('Plan Cancellation Request');
    const body = encodeURIComponent('Hello,\n\nI would like to cancel my current plan.\n\nThank you.');
    window.location.href = `mailto:support@getconvoy.io?subject=${subject}&body=${body}`;
  }

  onUpgradePlan() {
    const subject = encodeURIComponent('Plan Upgrade Request');
    const body = encodeURIComponent('Hello,\n\nI would like to upgrade to the Enterprise plan.\n\nThank you.');
    window.location.href = `mailto:support@getconvoy.io?subject=${subject}&body=${body}`;
  }

  selectPlan(planId: string) {
    this.selectedPlan = planId as 'pro' | 'enterprise';
  }

  getFeaturesByCategory(category: 'core' | 'security' | 'support'): any[] {
    if (this.plans.length === 0) return [];

    // Get all unique features for this category across all plans
    const allFeatures = this.plans.flatMap(plan =>
      plan.features.filter(feature => feature.category === category)
    );

    // Remove duplicates based on feature name
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
    this.loadPaymentMethodDetails();
    this.loadBillingAddress();
    this.loadVatInfo();
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

  private loadBillingAddress() {
    this.isLoadingBillingAddress = true;
    this.billingPaymentDetailsService.getBillingAddress().subscribe({
      next: (details) => {
        this.billingAddressDetails = details;
        this.isLoadingBillingAddress = false;
        console.log('Loaded billing address:', details);
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
    // Validate organisation before starting payment method flow
    try {
      const org = localStorage.getItem('CONVOY_ORG');
      if (!org) {
        throw new Error('No organisation found');
      }

      const orgData = JSON.parse(org);
      if (!orgData.uid) {
        throw new Error('Invalid organisation data');
      }

      console.log('Starting payment method flow with organisation:', orgData.uid);
    } catch (error) {
      console.error('Cannot start payment method flow - invalid organisation:', error);
      this.generalService.showNotification({
        message: 'Invalid organisation data. Please refresh the page and try again.',
        style: 'error'
      });
      return;
    }

    this.isEditingPaymentMethod = true;
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
        console.log('Setup intent received:', this.setupIntentSecret ? 'Success' : 'Failed');
      },
      error: (error) => {
        console.error('Failed to get setup intent:', error);
        console.error('Error details:', error);
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
    this.loadPaymentMethodDetails();
    this.refreshOverviewTrigger++;
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

  async onUpdatePaymentMethodWithProvider(stripeElementsComponent: StripeElementsComponent, event?: Event) {
    // This will be called from the template when using payment provider Elements
    // The actual confirmation happens in the StripeElementsComponent
    console.log('Save Card button clicked - triggering Stripe confirmation');
    console.log('Event:', event);
    console.log('stripeElementsComponent:', stripeElementsComponent);

    // Prevent any default form submission behavior
    if (event) {
      event.preventDefault();
      event.stopPropagation();
      console.log('Prevented default and stopped propagation');
    }

    this.isSavingPaymentMethod = true;
    console.log('Set isSavingPaymentMethod to true');

    // Add a small delay to see if the page reloads before this completes
    await new Promise(resolve => setTimeout(resolve, 100));
    console.log('After 100ms delay - still here');

    try {
      // Confirm the payment method with the existing setup intent
      console.log('Confirming payment method...');
      const success = await stripeElementsComponent.confirmPaymentMethod();
      if (success) {
        console.log('Payment method confirmed successfully!');
        this.onPaymentMethodCreated();
      } else {
        console.log('Payment method confirmation failed');
      }
    } catch (error) {
      console.error('Error confirming payment method:', error);
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
      console.log('Updating billing address:', this.billingAddressForm.value);

      this.billingPaymentDetailsService.updateBillingAddress(this.billingAddressForm.value).subscribe({
        next: (response) => {
          console.log('Billing address updated successfully:', response);
          this.generalService.showNotification({
            message: 'Billing address updated successfully!',
            style: 'success'
          });
          this.isEditingBillingAddress = false;
          this.loadBillingAddress();
        },
        error: (error) => {
          console.error('Failed to update billing address:', error);
          this.generalService.showNotification({
            message: 'Failed to update billing address. Please try again.',
            style: 'error'
          });
        }
      });
    } else {
      this.markFormGroupTouched(this.billingAddressForm);
    }
  }

  onUpdateVatInfo() {
    if (this.vatForm.valid) {
      console.log('Updating VAT info:', this.vatForm.value);

      this.billingPaymentDetailsService.updateVatInfo(this.vatForm.value).subscribe({
        next: (response) => {
          console.log('VAT information updated successfully:', response);
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

      // Check if it matches any known pattern
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

      // Check if it matches any known pattern
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
