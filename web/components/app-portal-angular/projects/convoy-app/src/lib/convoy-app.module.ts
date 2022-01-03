import { CommonModule } from '@angular/common';
import { NgModule } from '@angular/core';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { ConvoyAppComponent } from './convoy-app.component';
import { PrismModule } from './prism/prism.module';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatNativeDateModule } from '@angular/material/core';

@NgModule({
    declarations: [ConvoyAppComponent],
    imports: [
        CommonModule,
        PrismModule,
        FormsModule,
        ReactiveFormsModule,
        MatDatepickerModule,
        MatNativeDateModule,
    ],
    exports: [ConvoyAppComponent],
})
export class ConvoyAppModule {}
