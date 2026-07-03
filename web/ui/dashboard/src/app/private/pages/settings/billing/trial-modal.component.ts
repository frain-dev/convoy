import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output, ViewChild, ElementRef, OnDestroy } from '@angular/core';
import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
import { Subscription } from 'rxjs';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { DialogDirective, DialogHeaderComponent } from 'src/app/components/dialog/dialog.directive';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { HttpService } from 'src/app/services/http/http.service';
import { TrialStatusService } from 'src/app/services/trial-status/trial-status.service';
import { PlanCatalogDialogComponent } from './plan-catalog-dialog.component';
import { PlanService } from './plan.service';
import {
	formatTrialIntro,
	formatTrialLimitLine,
	hasTrialLimits,
	resolveTrialOffer,
	resolveTrialPlanName,
	trialFeaturesLead,
	TrialOffer
} from './trial-offer.util';

@Component({
	selector: 'convoy-trial-modal',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, DialogDirective, DialogHeaderComponent, InputDirective, InputFieldDirective, InputErrorComponent, LabelComponent, PlanCatalogDialogComponent],
	templateUrl: './trial-modal.component.html',
	styles: [`
		dialog.trial-dialog[open] { animation: trial-modal-in 180ms ease-out; }
		dialog.trial-dialog::backdrop { animation: trial-backdrop-in 180ms ease-out; }
		@keyframes trial-modal-in {
			from { opacity: 0; transform: translateY(8px) scale(0.985); }
			to { opacity: 1; transform: none; }
		}
		@keyframes trial-backdrop-in {
			from { opacity: 0; }
			to { opacity: 1; }
		}
	`]
})
export class TrialModalComponent implements OnDestroy {
	@Input() mode: 'cloud' | 'self_hosted' = 'cloud';
	@Input() selfHostedOffer: TrialOffer | null = null;

	@Output() trialStarted = new EventEmitter<void>();
	@Output() subscribeNow = new EventEmitter<void>();

	@ViewChild('dialog') dialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('planCatalogDialog') planCatalogDialog!: PlanCatalogDialogComponent;

	cloudOffer: TrialOffer | null = null;
	starting = false;
	billingEmail = new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.email] });
	formatTrialLimitLine = formatTrialLimitLine;
	trialFeaturesLead = trialFeaturesLead;

	private offerSub?: Subscription;

	constructor(
		private planService: PlanService,
		private trialStatusService: TrialStatusService,
		private generalService: GeneralService,
		private httpService: HttpService
	) {
		this.offerSub = this.trialStatusService.offer$.subscribe(offer => (this.cloudOffer = offer));
	}

	ngOnDestroy(): void {
		this.offerSub?.unsubscribe();
	}

	get orgId(): string {
		return this.httpService.getOrganisation()?.uid || '';
	}

	get effectiveOffer(): TrialOffer {
		return resolveTrialOffer(this.mode, this.cloudOffer, this.selfHostedOffer);
	}

	get trialIntro(): string {
		return formatTrialIntro(this.mode, this.cloudOffer, this.selfHostedOffer);
	}

	get resolvedPlanName(): string {
		return resolveTrialPlanName(this.mode, this.effectiveOffer);
	}

	get limits() {
		const list = this.effectiveOffer.limits ?? [];
		if (this.mode === 'self_hosted') {
			return list.filter((limit) => limit.key !== 'daily_event_limit');
		}
		return list;
	}

	get hasTrialLimits(): boolean {
		return hasTrialLimits(this.effectiveOffer);
	}

	get startTrialDisabled(): boolean {
		if (this.starting) {
			return true;
		}
		if (this.mode === 'self_hosted') {
			return this.billingEmail.invalid;
		}
		return false;
	}

	open(): void {
		this.starting = false;
		if (this.mode === 'self_hosted') {
			this.billingEmail.reset('');
		}
		this.dialog?.nativeElement.showModal();
	}

	close(): void {
		if (this.starting) return;
		this.dialog?.nativeElement.close();
	}

	onViewPlanFeatures(): void {
		if (this.starting) return;
		this.planCatalogDialog?.open();
	}

	onSubscribeNow(): void {
		if (this.starting) return;
		this.dialog?.nativeElement.close();
		this.subscribeNow.emit();
	}

	async onStartFreeTrial(): Promise<void> {
		if (this.starting) return;

		if (this.mode === 'self_hosted') {
			if (this.billingEmail.invalid) {
				this.billingEmail.markAsTouched();
				return;
			}
		}

		this.starting = true;

		try {
			if (this.mode === 'self_hosted') {
				await this.planService.startSelfHostedTrial(
					this.billingEmail.value.trim(),
					window.location.origin
				);
			} else {
				await this.planService.startTrial(this.orgId);
			}
		} catch (error: any) {
			this.starting = false;
			this.generalService.showNotification({
				message: error?.error?.message || 'Failed to start trial. Please try again.',
				style: 'error'
			});
			return;
		}

		this.starting = false;
		this.dialog?.nativeElement.close();
		this.trialStarted.emit();
	}
}
