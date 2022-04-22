import { CommonModule, DatePipe } from '@angular/common';
import { NgModule } from '@angular/core';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { ConvoyAppComponent } from './convoy-app.component';
import { PrismModule } from './prism/prism.module';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatNativeDateModule } from '@angular/material/core';
import { SvgComponent } from './shared-components/svg.component';
import { ConvoyTableLoaderComponent } from './shared-components/table-loader.component';

@NgModule({
    declarations: [ConvoyAppComponent, SvgComponent, ConvoyTableLoaderComponent],
    imports: [
        CommonModule,
        PrismModule,
        FormsModule,
        ReactiveFormsModule,
        MatDatepickerModule,
        MatNativeDateModule
    ],
    exports: [ConvoyAppComponent],
    providers: [DatePipe]
})
export class ConvoyAppModule {}
