import { CommonModule } from '@angular/common';
import { Component, Input, OnChanges, OnInit, SimpleChanges } from '@angular/core';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { PlanCatalogService } from './plan-catalog.service';
import {
	comparisonTableGridColumns,
	getFeatureValue,
	getFeatureValueType,
	getFeaturesByCategory
} from './plan-comparison.util';
import { Plan, PlanFeature, PlanService } from './plan.service';
import { filterPlansToTrialPlan } from './trial-offer.util';

@Component({
	selector: 'convoy-plan-catalog-preview',
	standalone: true,
	imports: [CommonModule, SkeletonLoaderComponent],
	templateUrl: './plan-catalog-preview.component.html',
	styleUrls: ['./plan-catalog-preview.component.scss']
})
export class PlanCatalogPreviewComponent implements OnInit, OnChanges {
	@Input({ required: true }) mode!: 'cloud' | 'self_hosted';
	/** When set, show only the trial SKU (Premium / Cloud Pro), not the full catalog. */
	@Input() trialPlanName: string | null = null;
	@Input() managedExternally = false;
	@Input() plans: Plan[] = [];
	@Input() isLoadingPlans = false;
	@Input() hasLoadedPlans = false;
	@Input() hasAttemptedPlansLoad = false;
	@Input() plansUnavailableMessage = '';

	displayPlans: Plan[] = [];
	catalogPlans: Plan[] = [];
	internalLoading = false;
	internalHasLoaded = false;
	internalAttempted = false;
	internalUnavailableMessage = '';

	readonly planCategories: Array<'core' | 'security' | 'support'> = ['core', 'security', 'support'];

	constructor(
		private planService: PlanService,
		private planCatalog: PlanCatalogService
	) {}

	ngOnInit(): void {
		if (this.managedExternally) {
			this.applyCatalogPlans(this.plans);
			return;
		}
		void this.loadPlans();
	}

	ngOnChanges(changes: SimpleChanges): void {
		if (this.managedExternally && (changes['plans'] || changes['managedExternally'])) {
			this.applyCatalogPlans(this.plans);
		}
		if (changes['trialPlanName'] && this.catalogPlans.length > 0) {
			this.applyVisiblePlans();
		}
	}

	get visiblePlans(): Plan[] {
		return this.displayPlans;
	}

	get singlePlanView(): boolean {
		return !!this.trialPlanName && this.displayPlans.length === 1;
	}

	get featuresSectionTitle(): string {
		return this.singlePlanView ? 'Features included' : 'Compare plans';
	}

	private applyCatalogPlans(plans: Plan[]): void {
		this.catalogPlans = plans;
		this.applyVisiblePlans();
	}

	private applyVisiblePlans(): void {
		this.displayPlans = this.trialPlanName
			? filterPlansToTrialPlan(this.catalogPlans, this.trialPlanName)
			: this.catalogPlans;
	}

	categoryTitle(category: 'core' | 'security' | 'support'): string {
		switch (category) {
			case 'core':
				return 'Core Capabilities';
			case 'security':
				return 'Security & Compliance';
			case 'support':
				return 'Support';
		}
	}

	get comparisonGridColumns(): string {
		return comparisonTableGridColumns(this.displayPlans.length);
	}

	get loadingPlans(): boolean {
		return this.managedExternally ? this.isLoadingPlans : this.internalLoading;
	}

	get readyState(): boolean {
		const loaded = this.managedExternally ? this.hasLoadedPlans : this.internalHasLoaded;
		return !this.loadingPlans && loaded && this.displayPlans.length > 0;
	}

	get loadingState(): boolean {
		return this.loadingPlans;
	}

	get unavailableState(): boolean {
		const attempted = this.managedExternally ? this.hasAttemptedPlansLoad : this.internalAttempted;
		const loaded = this.managedExternally ? this.hasLoadedPlans : this.internalHasLoaded;
		return !this.loadingPlans && attempted && !loaded;
	}

	get unavailableMessage(): string {
		return this.managedExternally ? this.plansUnavailableMessage : this.internalUnavailableMessage;
	}

	featuresByCategory(category: 'core' | 'security' | 'support'): PlanFeature[] {
		return getFeaturesByCategory(this.displayPlans, category);
	}

	featureValue(planId: string, featureName: string): string {
		return getFeatureValue(this.displayPlans, planId, featureName);
	}

	featureValueType(planId: string, featureName: string): 'supported' | 'unsupported' | 'plain' {
		return getFeatureValueType(this.displayPlans, planId, featureName);
	}

	/** Plan card highlights: supported (or non-Unsupported) features only. */
	cardHighlightFeatures(plan: Plan, limit = 3): PlanFeature[] {
		return plan.features
			.filter(feature => (feature.value || '').toLowerCase() !== 'unsupported')
			.slice(0, limit);
	}

	private async loadPlans(): Promise<void> {
		this.internalLoading = true;
		this.internalAttempted = true;
		this.internalHasLoaded = false;
		this.internalUnavailableMessage = '';

		try {
			const response = await new Promise<{ data?: Plan[] }>((resolve, reject) => {
				this.planService.getPlans().subscribe({
					next: resolve,
					error: reject
				});
			});
			const defaultPlans = this.planService.getDefaultPlanComparison(this.mode === 'self_hosted').plans;
			const plansFromApi = Array.isArray(response.data) ? response.data : [];
			const isSelfHostedBilling = this.mode === 'self_hosted';
			const catalog = this.planCatalog.buildCatalog(plansFromApi, defaultPlans, isSelfHostedBilling);
			this.applyCatalogPlans(catalog.plans);
			this.internalHasLoaded = this.displayPlans.length > 0;
			this.internalUnavailableMessage = catalog.plansUnavailableMessage;
		} catch {
			this.catalogPlans = [];
			this.displayPlans = [];
			this.internalUnavailableMessage = 'Plans could not be loaded. Please try again later.';
		} finally {
			this.internalLoading = false;
		}
	}
}
