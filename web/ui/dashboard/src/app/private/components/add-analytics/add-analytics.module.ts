import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AddAnalyticsComponent } from './add-analytics.component';
import { ReactiveFormsModule } from '@angular/forms';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';

@NgModule({
	declarations: [AddAnalyticsComponent],
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, RadioComponent, ModalComponent],
	exports: [AddAnalyticsComponent]
})
export class AddAnalyticsModule {}
