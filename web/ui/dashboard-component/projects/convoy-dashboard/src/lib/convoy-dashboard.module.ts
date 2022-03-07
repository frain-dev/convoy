import { NgModule } from '@angular/core';
import { ConvoyDashboardComponent } from './convoy-dashboard.component';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatNativeDateModule } from '@angular/material/core';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { PrismModule } from './prism/prism.module';
import { ConvoyLoaderComponent } from './loader-component/loader.component';
import { SharedModule } from './shared/shared.module';

@NgModule({
	declarations: [ConvoyDashboardComponent, ConvoyLoaderComponent],
	imports: [CommonModule, MatDatepickerModule, MatNativeDateModule, FormsModule, ReactiveFormsModule, PrismModule, SharedModule],
	exports: [ConvoyDashboardComponent]
})
export class ConvoyDashboardModule {}
