import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateProjectComponent } from './create-project-component.component';
import { ReactiveFormsModule } from '@angular/forms';
import { TooltipModule } from '../tooltip/tooltip.module';

@NgModule({
	declarations: [CreateProjectComponent],
	imports: [CommonModule, ReactiveFormsModule, TooltipModule],
	exports: [CreateProjectComponent]
})
export class CreateProjectComponentModule {}
