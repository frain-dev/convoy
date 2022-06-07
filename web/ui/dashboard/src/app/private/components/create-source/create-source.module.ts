import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateSourceComponent } from './create-source.component';
import { TooltipModule } from '../tooltip/tooltip.module';
import { ReactiveFormsModule } from '@angular/forms';

@NgModule({
	declarations: [CreateSourceComponent],
	imports: [CommonModule, TooltipModule, ReactiveFormsModule],
	exports: [CreateSourceComponent]
})
export class CreateSourceModule {}
