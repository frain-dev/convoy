import { CommonModule } from '@angular/common';
import { Component, ElementRef, Input, ViewChild } from '@angular/core';
import { DialogDirective, DialogHeaderComponent } from 'src/app/components/dialog/dialog.directive';
import { PlanCatalogPreviewComponent } from './plan-catalog-preview.component';

@Component({
	selector: 'convoy-plan-catalog-dialog',
	standalone: true,
	imports: [CommonModule, DialogDirective, DialogHeaderComponent, PlanCatalogPreviewComponent],
	templateUrl: './plan-catalog-dialog.component.html',
	styleUrls: ['./plan-catalog-dialog.component.scss']
})
export class PlanCatalogDialogComponent {
	@Input({ required: true }) mode!: 'cloud' | 'self_hosted';
	@Input() trialPlanName: string | null = null;

	@ViewChild('dialog') dialog!: ElementRef<HTMLDialogElement>;

	open(): void {
		this.dialog?.nativeElement.showModal();
	}

	close(): void {
		this.dialog?.nativeElement.close();
	}
}
