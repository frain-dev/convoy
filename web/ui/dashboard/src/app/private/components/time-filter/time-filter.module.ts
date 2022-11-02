import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TimeFilterComponent } from './time-filter.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { FormsModule } from '@angular/forms';

@NgModule({
	declarations: [TimeFilterComponent],
	imports: [CommonModule, DropdownComponent, ButtonComponent, FormsModule],
	exports: [TimeFilterComponent]
})
export class TimeFilterModule {}
