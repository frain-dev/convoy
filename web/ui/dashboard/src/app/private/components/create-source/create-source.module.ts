import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateSourceComponent } from './create-source.component';
import { TooltipModule } from '../tooltip/tooltip.module';

@NgModule({
	declarations: [CreateSourceComponent],
	imports: [CommonModule, TooltipModule],
	exports: [CreateSourceComponent]
})
export class CreateSourceModule {}
