import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AddAnalyticsComponent } from './add-analytics.component';
import { ReactiveFormsModule } from '@angular/forms';
import { TooltipModule } from '../tooltip/tooltip.module';

@NgModule({
	declarations: [AddAnalyticsComponent],
	imports: [CommonModule, ReactiveFormsModule, TooltipModule],
	exports: [AddAnalyticsComponent]
})
export class AddAnalyticsModule {}
