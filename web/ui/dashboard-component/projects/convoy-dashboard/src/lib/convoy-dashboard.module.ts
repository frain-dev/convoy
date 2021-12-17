import { NgModule } from '@angular/core';
import { ConvoyDashboardComponent } from './convoy-dashboard.component';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatNativeDateModule } from '@angular/material/core';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';

@NgModule({
	declarations: [ConvoyDashboardComponent],
	imports: [CommonModule, MatDatepickerModule, MatNativeDateModule, FormsModule, ReactiveFormsModule],
	exports: [ConvoyDashboardComponent]
})
export class ConvoyDashboardModule {}
