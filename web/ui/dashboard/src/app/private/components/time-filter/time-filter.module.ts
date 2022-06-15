import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TimeFilterComponent } from './time-filter.component';

@NgModule({
	declarations: [TimeFilterComponent],
	imports: [CommonModule],
	exports: [TimeFilterComponent]
})
export class TimeFilterModule {}
