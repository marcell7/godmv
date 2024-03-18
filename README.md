# ⚡ godmv - hitro procesiranje DMV podatkov


DMV (digitalni model višin) podatki so dostopni na GURS. Več info o tem, kaj to je: https://www.e-prostor.gov.si/podrocja/drzavni-topografski-sistem/digitalni-modeli-visin/?acitem=1220-1221

Teh podatkov je precej. Razdeljeni so po >3000 tekstovnih datotekah. Vse datoteke skupaj so velike ~16GB, kar pomeni približno ~800.000.000 vrstic. Problem se pojavi, ko želiš izvoziti podatke le za določen del Slovenije. 
- Prvič ne veš, v kateri datoteki se podatki za ta del nahajajo.
- Drugič, teh podatkov je preveč, da bi procesiral vse naenkrat. 

To je program, ki hitro obdela vse datoteke/podatke (na mojem PCju traja ~15s) in zgenerira eno csv datoteko, ki vsebuje samo podatke samo za tisto območje, ki ga določimo na začetku. Poleg tega lahko izberemo možnost, da podatke prenesemo iz GURSa, če jih še nimamo. Prenesejo pa se vsi podatki.

Zadeva zaenkrat deluje izključno na Linuxih (testirano na Ubuntu), ker se uporablja knjižnica Proj (za pretvorbo koordinat):
Za delovanje preveri, da imaš inštalirano:
```bash
sudo apt install lib-geoproj
```

Poženi program s poljubnim bbox parametrom
```bash
./godmv --bbox="46.3746;46.3812;13.8365;13.8538" --download=true
```

Parameter bbox predstavlja robne koordinate območja, ki ga želimo izvoziti. Program nato izvozi vse zapise, ki se nahajajo znotraj tega območja.
Vrstni red koordinat v bbox parametru je sledeč:
```bash
                2.
         |---46.3812---| 
         |             |
   3. 13.8365       13.8538  4.
         |             |
         |---46.3764---|
                1.
```

Help:
```bash
./godmv --help
```
